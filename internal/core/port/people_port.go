package port

import (
	"context"

	"github.com/putteror/sea-value/internal/core/model"
)

// PeopleAPIPort handles uploading people excel files or batch importing users into CentAccess
type PeopleAPIPort interface {
	UploadPeopleExcel(ctx context.Context, filePath string, personType string) (*model.ImportResult, error)
	ImportPeople(ctx context.Context, people []model.PeopleImportRow) (*model.ImportResult, error)
}

// ExcelReaderPort handles scanning and parsing Excel files from people folder
type ExcelReaderPort interface {
	ReadPeopleExcel(filePath string) ([]model.PeopleImportRow, error)
	ListPendingPeopleFiles() ([]string, error)
	MoveProcessedFile(filePath string) error
}
