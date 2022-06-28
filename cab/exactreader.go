package cab

import "io"

// ExactReader returns a Reader that reads from r
// but stops with EOF after n bytes. It returns ErrUnexpectedEOF if
// the underlying reader returns EOF before n bytes.
// The underlying implementation is a *ExactReader.
func ExactReader(r io.Reader, n int64) io.ReadCloser { return &ExactReaderImpl{r, n} }

// A ExactReaderImpl reads from R but limits the amount of
// data returned to just N bytes. Each call to Read
// updates N to reflect the new amount remaining.
// Read returns EOF when N <= 0 or when the underlying R returns EOF.
type ExactReaderImpl struct {
	R io.Reader // underlying reader
	N int64     // max bytes remaining
}

func (e *ExactReaderImpl) Read(p []byte) (n int, err error) {
	if e.N <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > e.N {
		p = p[0:e.N]
	}
	n, err = e.R.Read(p)
	e.N -= int64(n)
	if err == io.EOF && e.N > 0 {
		err = io.ErrUnexpectedEOF
	}
	return
}

func (e *ExactReaderImpl) Close() error {
	_, err := io.Copy(io.Discard, e)
	return err
}
