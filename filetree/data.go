package filetree

import (
	"archive/tar"
	"fmt"
	"io"

	"github.com/cespare/xxhash"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	Unchanged DiffType = iota
	Changed
	Added
	Removed
)

// NodeData is the payload for a FileNode
type NodeData struct {
	ViewInfo ViewInfo
	FileInfo FileInfo
	DiffType DiffType
}

// ViewInfo contains UI specific detail for a specific FileNode
type ViewInfo struct {
	Collapsed bool
	Hidden    bool
}

// FileInfo contains tar metadata for a specific FileNode
type FileInfo struct {
	Path      string
	TypeFlag  byte
	hash      uint64
	TarHeader tar.Header
}

// DiffType defines the comparison result between two FileNodes
type DiffType int

// NewNodeData creates an empty NodeData struct for a FileNode
func NewNodeData() *NodeData {
	return &NodeData{
		ViewInfo: *NewViewInfo(),
		FileInfo: FileInfo{},
		DiffType: Unchanged,
	}
}

// Copy duplicates a NodeData
func (data *NodeData) Copy() *NodeData {
	return &NodeData{
		ViewInfo: *data.ViewInfo.Copy(),
		FileInfo: *data.FileInfo.Copy(),
		DiffType: data.DiffType,
	}
}

// NewViewInfo creates a default ViewInfo
func NewViewInfo() (view *ViewInfo) {
	return &ViewInfo{
		Collapsed: viper.GetBool("filetree.collapse-dir"),
		Hidden:    false,
	}
}

// Copy duplicates a ViewInfo
func (view *ViewInfo) Copy() (newView *ViewInfo) {
	newView = NewViewInfo()
	*newView = *view
	return newView
}

var chuckSize = 2 * 1024 * 1024

func getHashFromReader(reader io.Reader) (uint64, uint64) {
	h := xxhash.New()

	buf := make([]byte, chuckSize)
	var bytesRead uint64

	bytesRead = 0
	for {
		n, err := reader.Read(buf)
		bytesRead += uint64(n)
		if err != nil && err != io.EOF {
			logrus.Panic(err)
		}
		if n == 0 {
			break
		}

		h.Write(buf[:n])
	}

	return h.Sum64(), bytesRead
}

// NewFileInfo extracts the metadata from a tar header and file contents and generates a new FileInfo object.
func NewFileInfo(reader *tar.Reader, header *tar.Header, path string) FileInfo {
	if header.Typeflag == tar.TypeDir {
		return FileInfo{
			Path:      path,
			TypeFlag:  header.Typeflag,
			hash:      0,
			TarHeader: *header,
		}
	}

	fmt.Printf("Reading %v(%v)...\n", path, header.Size)
	hash, bytesRead := getHashFromReader(reader)
	if bytesRead != uint64(header.Size) {
		fmt.Printf("NewFileInfo: Not enough bytes in '%v': %v (%v expected)\n", path, bytesRead, header.Size)

		logrus.Panic()
	}
	return FileInfo{
		Path:      path,
		TypeFlag:  header.Typeflag,
		hash:      hash,
		TarHeader: *header,
	}
}

// Copy duplicates a FileInfo
func (data *FileInfo) Copy() *FileInfo {
	if data == nil {
		return nil
	}
	return &FileInfo{
		Path:      data.Path,
		TypeFlag:  data.TypeFlag,
		hash:      data.hash,
		TarHeader: data.TarHeader,
	}
}

// Compare determines the DiffType between two FileInfos based on the type and contents of each given FileInfo
func (data *FileInfo) Compare(other FileInfo) DiffType {
	if data.TypeFlag == other.TypeFlag {
		if data.hash == other.hash {
			return Unchanged
		}
	}
	return Changed
}

// String of a DiffType
func (diff DiffType) String() string {
	switch diff {
	case Unchanged:
		return "Unchanged"
	case Changed:
		return "Changed"
	case Added:
		return "Added"
	case Removed:
		return "Removed"
	default:
		return fmt.Sprintf("%d", int(diff))
	}
}

// merge two DiffTypes into a single result. Essentially, return the given value unless they two values differ,
// in which case we can only determine that there is "a change".
func (diff DiffType) merge(other DiffType) DiffType {
	if diff == other {
		return diff
	}
	return Changed
}
