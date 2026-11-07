package fsutil

import "io"

// MultiCloser closes several resources (an archive entry and the archive).
type MultiCloser []io.Closer

func MultiClose(closers MultiCloser) {
	for _, closer := range closers {
		if closer != nil {
			closer.Close()
		}
	}
}
