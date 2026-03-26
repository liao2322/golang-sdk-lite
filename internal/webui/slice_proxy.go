package webui

import (
	"context"
	"crypto/sha1"
	"fmt"
	"hash"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/halalcloud/golang-sdk-lite/halalcloud/services/userfile"
)

type chunkSize struct {
	position int64
	size     int
}

type openListReader struct {
	ctx     context.Context
	mu      sync.Mutex
	addrs   []*userfile.SliceDownloadInfo
	id      int
	skip    int64
	chunk   []byte
	chunks  []chunkSize
	closed  bool
	sha     string
	shaTemp hash.Hash
}

func serveOpenListProxy(w http.ResponseWriter, r *http.Request, svc *userfile.UserFileService, identity string) error {
	fileInfo, err := svc.Get(r.Context(), &userfile.File{Identity: identity})
	if err != nil {
		return err
	}

	parsed, err := svc.ParseFileSlice(r.Context(), &userfile.File{Identity: identity})
	if err != nil {
		return err
	}

	size, _ := strconv.ParseInt(parsed.FileSize, 10, 64)
	addrs, expireAt, err := getAllSliceAddresses(r.Context(), svc, parsed.RawNodes)
	if err != nil {
		return err
	}

	_ = expireAt
	w.Header().Set("Accept-Ranges", "bytes")
	if fileInfo.MimeType != "" {
		w.Header().Set("Content-Type", fileInfo.MimeType)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	if fileInfo.Name != "" {
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", strings.ReplaceAll(fileInfo.Name, "\"", "")))
	}

	if r.Method == http.MethodHead {
		if size > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
		}
		w.WriteHeader(http.StatusOK)
		return nil
	}

	start, end, status, err := parseRangeHeader(r.Header.Get("Range"), size)
	if err != nil {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
		http.Error(w, err.Error(), http.StatusRequestedRangeNotSatisfiable)
		return nil
	}

	length := end - start + 1
	if status == http.StatusPartialContent {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
	}
	if length >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	}
	w.WriteHeader(status)

	reader := &openListReader{
		ctx:     r.Context(),
		addrs:   addrs,
		chunk:   []byte{},
		chunks:  getChunkSizes(parsed.Sizes),
		skip:    start,
		sha:     parsed.Sha1,
		shaTemp: sha1.New(),
	}
	defer reader.Close()

	_, copyErr := io.CopyN(w, reader, length)
	if copyErr != nil && copyErr != io.EOF {
		return copyErr
	}
	return nil
}

func getAllSliceAddresses(ctx context.Context, svc *userfile.UserFileService, rawNodes []string) ([]*userfile.SliceDownloadInfo, int64, error) {
	fileAddrs := make([]*userfile.SliceDownloadInfo, 0, len(rawNodes))
	var addressDuration int64

	nodesNumber := len(rawNodes)
	nodesIndex := nodesNumber - 1
	startIndex, endIndex := 0, nodesIndex
	for nodesIndex >= 0 {
		if nodesIndex >= 200 {
			endIndex = 200
		} else {
			endIndex = nodesNumber
		}
		for ; endIndex <= nodesNumber; endIndex += 200 {
			if endIndex == 0 {
				endIndex = 1
			}
			sliceAddress, err := svc.GetSliceDownloadAddress(ctx, &userfile.SliceDownloadAddressRequest{
				Identity: rawNodes[startIndex:endIndex],
				Version:  1,
			})
			if err != nil {
				return nil, 0, err
			}
			addressDuration, _ = strconv.ParseInt(sliceAddress.ExpireAt, 10, 64)
			fileAddrs = append(fileAddrs, sliceAddress.Addresses...)
			startIndex = endIndex
			nodesIndex -= 200
		}
	}

	return fileAddrs, addressDuration, nil
}

func getChunkSizes(sliceSize []*userfile.SliceSize) []chunkSize {
	chunks := make([]chunkSize, 0)
	for _, s := range sliceSize {
		endIndex := s.EndIndex
		startIndex := s.StartIndex
		if endIndex == 0 {
			endIndex = startIndex
		}
		for j := startIndex; j <= endIndex; j++ {
			chunks = append(chunks, chunkSize{position: j, size: int(s.Size)})
		}
	}
	return chunks
}

func (oo *openListReader) getChunk() error {
	if oo.id >= len(oo.chunks) || oo.id >= len(oo.addrs) {
		return io.EOF
	}
	chunk, err := getRawFile(oo.addrs[oo.id])
	if err != nil {
		return err
	}
	oo.id++
	oo.chunk = chunk
	return nil
}

func (oo *openListReader) Read(p []byte) (n int, err error) {
	oo.mu.Lock()
	defer oo.mu.Unlock()
	if oo.closed {
		return 0, fmt.Errorf("read on closed file")
	}
	for oo.skip > 0 {
		_, size, chunkErr := oo.chunkLocation(oo.id)
		if chunkErr != nil {
			return 0, chunkErr
		}
		if oo.skip < int64(size) {
			break
		}
		oo.id++
		oo.skip -= int64(size)
	}
	if len(oo.chunk) == 0 {
		if err = oo.getChunk(); err != nil {
			return 0, err
		}
		if oo.skip > 0 {
			oo.chunk = oo.chunk[oo.skip:]
			oo.skip = 0
		}
	}
	n = copy(p, oo.chunk)
	_, _ = oo.shaTemp.Write(p[:n])
	oo.chunk = oo.chunk[n:]
	return n, nil
}

func (oo *openListReader) Close() error {
	oo.mu.Lock()
	defer oo.mu.Unlock()
	if oo.closed {
		return nil
	}
	oo.closed = true
	return nil
}

func (oo *openListReader) chunkLocation(id int) (position int64, size int, err error) {
	if id < 0 || id >= len(oo.chunks) {
		return 0, 0, fmt.Errorf("invalid chunk index")
	}
	return oo.chunks[id].position, oo.chunks[id].size, nil
}

func getRawFile(addr *userfile.SliceDownloadInfo) ([]byte, error) {
	if addr == nil {
		return nil, fmt.Errorf("slice address is nil")
	}
	req, err := http.NewRequest(http.MethodGet, addr.DownloadAddress, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}
	if addr.Encrypt > 0 {
		cd := uint8(addr.Encrypt)
		for i := 0; i < len(body); i++ {
			body[i] ^= cd
		}
	}
	return body, nil
}

func parseRangeHeader(value string, size int64) (start, end int64, status int, err error) {
	if size < 0 {
		size = 0
	}
	if value == "" {
		if size == 0 {
			return 0, -1, http.StatusOK, nil
		}
		return 0, size - 1, http.StatusOK, nil
	}
	if !strings.HasPrefix(value, "bytes=") {
		return 0, 0, 0, fmt.Errorf("invalid range")
	}
	part := strings.TrimPrefix(value, "bytes=")
	if strings.Contains(part, ",") {
		return 0, 0, 0, fmt.Errorf("multiple ranges not supported")
	}
	segments := strings.SplitN(part, "-", 2)
	if len(segments) != 2 {
		return 0, 0, 0, fmt.Errorf("invalid range")
	}
	if segments[0] == "" {
		suffix, convErr := strconv.ParseInt(segments[1], 10, 64)
		if convErr != nil || suffix <= 0 {
			return 0, 0, 0, fmt.Errorf("invalid range")
		}
		if suffix > size {
			suffix = size
		}
		return size - suffix, size - 1, http.StatusPartialContent, nil
	}
	start, err = strconv.ParseInt(segments[0], 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, 0, fmt.Errorf("invalid range")
	}
	if segments[1] == "" {
		return start, size - 1, http.StatusPartialContent, nil
	}
	end, err = strconv.ParseInt(segments[1], 10, 64)
	if err != nil || end < start {
		return 0, 0, 0, fmt.Errorf("invalid range")
	}
	if end >= size {
		end = size - 1
	}
	return start, end, http.StatusPartialContent, nil
}
