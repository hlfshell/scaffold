package logs

import (
	"fmt"
	"io"
	"strings"
	"sync"
)

/*
LogStreams is a named collection of log readers.
*/
type LogStreams map[string]io.ReadCloser

/*
GetStream returns a named log stream. Multiple names are joined with ".",
so GetStream("data", "postgres") looks up "data.postgres".
*/
func (streams LogStreams) GetStream(names ...string) (io.ReadCloser, bool) {
	key := streamKey(names...)
	if key == "" {
		return nil, false
	}

	stream, ok := streams[key]
	return stream, ok
}

/*
Merge returns a single reader containing every stream in the collection.
Closing the returned reader closes every source stream.
*/
func (streams LogStreams) Merge() io.ReadCloser {
	return Merge(streams)
}

/*
Close closes every stream in the collection.
*/
func (streams LogStreams) Close() error {
	var firstErr error
	for _, stream := range streams {
		if stream == nil {
			continue
		}
		if err := stream.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

/*
Merge fans named log streams into one reader. Closing the returned reader
closes every source stream.
*/
func Merge(streams LogStreams) io.ReadCloser {
	reader, writer := io.Pipe()
	merged := &mergedReader{
		PipeReader: reader,
		closers:    make([]io.Closer, 0, len(streams)),
	}

	var waitGroup sync.WaitGroup
	for _, stream := range streams {
		if stream == nil {
			continue
		}

		merged.closers = append(merged.closers, stream)
		waitGroup.Add(1)
		go func(stream io.ReadCloser) {
			defer waitGroup.Done()
			_, _ = io.Copy(writer, stream)
		}(stream)
	}

	go func() {
		waitGroup.Wait()
		_ = writer.Close()
	}()

	return merged
}

/*
UniqueName returns name unless it already exists, in which case it adds a
numeric suffix.
*/
func UniqueName(streams LogStreams, name string) string {
	if _, ok := streams[name]; !ok {
		return name
	}

	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", name, i)
		if _, ok := streams[candidate]; !ok {
			return candidate
		}
	}
}

type mergedReader struct {
	*io.PipeReader
	closers []io.Closer
	once    sync.Once
}

func (r *mergedReader) Close() error {
	var firstErr error
	r.once.Do(func() {
		for _, closer := range r.closers {
			if err := closer.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}

		if err := r.PipeReader.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	})

	return firstErr
}

func streamKey(names ...string) string {
	parts := []string{}
	for _, name := range names {
		name = strings.Trim(name, ".")
		if name != "" {
			parts = append(parts, name)
		}
	}

	return strings.Join(parts, ".")
}
