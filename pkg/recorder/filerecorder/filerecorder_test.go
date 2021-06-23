package filerecorder

import (
	"fmt"
	"testing"
	"time"

	"github.com/openshift/insights-operator/pkg/record"
)

func getMemoryRecords(m int) record.MemoryRecords {
	var records record.MemoryRecords
	for i := 0; i < m; i++ {
		records = append(records, record.MemoryRecord{
			Name: fmt.Sprintf("config/mock%d", i),
			At:   time.Now(),
			Data: []byte("data"),
		})
	}
	return records
}

func newFileRecorder() FileRecorder {
	return FileRecorder{basePath: "/tmp"}
}

func Benchmark_FileRecorder(b *testing.B) {
	for i := 0; i < b.N; i++ {
		fr := newFileRecorder()
		records := getMemoryRecords(1000)
		fr.Save(records)
		fr.Prune(time.Now())
	}
}
