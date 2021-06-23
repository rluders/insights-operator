package filerecorder

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/openshift/insights-operator/pkg/record"
	"k8s.io/klog/v2"
)

type FileRecorder struct {
	basePath string
}

// New filerecorder driver
func New(path string) *FileRecorder {
	return &FileRecorder{basePath: path}
}

// Save the records into the archive
func (d *FileRecorder) Save(records record.MemoryRecords) (record.MemoryRecords, error) {
	wrote := 0
	start := time.Now()
	defer func() {
		if wrote > 0 {
			klog.V(2).Infof("Wrote %d records to disk in %s", wrote, time.Since(start).Truncate(time.Millisecond))
		}
	}()

	age := records[0].At.UTC()
	name := fmt.Sprintf("insights-%s", age.Format("2006-01-02-150405"))
	path := filepath.Join(d.basePath, name)

	completed := make([]record.MemoryRecord, 0, len(records))
	defer func() {
		wrote = len(completed)
	}()

	klog.V(4).Infof("Writing %d records to %s", len(records), path)

	var wg sync.WaitGroup

	for idx := range records {
		wg.Add(1)
		record := records[idx]
		go func() {
			defer wg.Done()
			recordFile(path, &record)
		}()
		completed = append(completed, record)
	}

	wg.Wait()

	err := compressFolder(name, path)
	if err != nil {
		klog.Errorf("Failed to compress: %v", err)
		return nil, err
	}

	return completed, nil
}

func recordFile(path string, rec *record.MemoryRecord) {
	path = filepath.Join(path, rec.Name)
	// ensure that the path exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(path), 0755)
	}
	_, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0640)
	if err != nil {
		klog.Errorf("Unable to create the record file %s: %v", rec.Name, err)
	}
}

func compressFolder(name, path string) error {
	archivePath := fmt.Sprintf("%s.%s", path, "tar.gz")
	f, err := os.OpenFile(archivePath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0640)
	if err != nil {
		if os.IsExist(err) {
			klog.Errorf("Tried to copy to %s which already exists", name)
			return err
		}
		return fmt.Errorf("unable to create archive: %v", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	filepath.Walk(path, func(file string, fi os.FileInfo, err error) error {
		// generate tar header
		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}

		// must provide real name
		// (see https://golang.org/src/archive/tar/common.go?#L626)
		header.Name = filepath.ToSlash(file)

		// write header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// if not a dir, write file content
		if !fi.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tw, data); err != nil {
				return err
			}
		}
		return nil
	})

	if err := tw.Close(); err != nil {
		return fmt.Errorf("unable to close tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		return fmt.Errorf("unable to close gzip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("unable to close file: %v", err)
	}

	// remove directory created to store the files
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("unable to remove the archive directory: %v", err)
	}

	return nil
}

// Prune the archives older than given time
func (d *FileRecorder) Prune(olderThan time.Time) error {
	files, err := ioutil.ReadDir(d.basePath)
	if err != nil {
		return err
	}
	count := 0
	var errors []string
	for _, file := range files {
		if isNotArchiveFile(file) {
			continue
		}
		if file.ModTime().After(olderThan) {
			continue
		}
		if err := os.Remove(filepath.Join(d.basePath, file.Name())); err != nil {
			errors = append(errors, err.Error())
			continue
		}
		count++
	}
	if len(errors) == 1 {
		return fmt.Errorf("failed to delete expired file: %v", errors[0])
	}
	if len(errors) > 1 {
		return fmt.Errorf("failed to delete %d expired files: %v", len(errors), errors[0])
	}
	if count > 0 {
		klog.V(4).Infof("Deleted %d files older than %s", count, olderThan.UTC().Format(time.RFC3339))
	}
	return nil
}

// Summary implements summarizer interface to insights uploader
func (d *FileRecorder) Summary(_ context.Context, since time.Time) (io.ReadCloser, bool, error) {
	files, err := ioutil.ReadDir(d.basePath)
	if err != nil {
		return nil, false, err
	}
	if len(files) == 0 {
		return nil, false, nil
	}
	recentFiles := make([]string, 0, len(files))
	for _, file := range files {
		if isNotArchiveFile(file) {
			continue
		}
		if !file.ModTime().After(since) {
			continue
		}
		recentFiles = append(recentFiles, file.Name())
	}
	if len(recentFiles) == 0 {
		return nil, false, nil
	}
	lastFile := recentFiles[len(recentFiles)-1]
	klog.V(4).Infof("Found files to send: %v", lastFile)
	f, err := os.Open(filepath.Join(d.basePath, lastFile))
	if err != nil {
		return nil, false, nil
	}
	return f, true, nil
}

func isNotArchiveFile(file os.FileInfo) bool {
	return file.IsDir() || !strings.HasPrefix(file.Name(), "insights-") || !strings.HasSuffix(file.Name(), ".tar.gz")
}
