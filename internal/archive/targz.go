package archive

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
)

func Extract(r io.Reader, path string) error {
	uncompressedStream, err := gzip.NewReader(r)
	if err != nil {
		return err
	}

	tarReader := tar.NewReader(uncompressedStream)
	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			err := os.Mkdir(path+"/"+header.Name, 0755)
			if err != nil {
				return err
			}
		case tar.TypeReg:
			err := copyFile(tarReader, path+"/"+header.Name)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("uknown tar header type: %v in %s", header.Typeflag, header.Name)
		}
	}

	return nil
}

func copyFile(tr *tar.Reader, dest string) error {
	o, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer o.Close()
	_, err = io.Copy(o, tr)
	return err
}
