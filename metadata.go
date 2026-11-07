package main

import (
	"github.com/blamoo/sugoi/internal/metadata"
)

// Temporary aliases while the code moves into internal packages.
type FileMetadataStatic = metadata.Static
type FileMetadataDynamic = metadata.Dynamic
type StringArray = metadata.StringArray
type IdArray = metadata.IdArray

// fillStaticPlaceholders fills an empty title/collection from the file name and
// the page count from the archive. Applied to in-memory copies only.
func fillStaticPlaceholders(fms *FileMetadataStatic, file *FilePointer) {
	if file == nil {
		return
	}

	if len(fms.Title) == 0 {
		fms.Title = file.PlaceholderTitle()
	}

	if len(fms.Collection) == 0 {
		fms.Collection = file.PlaceholderCollection()
	}

	if fms.Pages == 0 {
		f, err := filePointers.Pages(file)
		if err == nil {
			fms.Pages = len(f)
		}
	}
}
