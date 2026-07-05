package dto

type UploadVideoData struct {
	VideoID uint64 `json:"video_id"`
	TaskID  string `json:"task_id"`
	RawURL  string `json:"raw_url"`
	HLSURL  string `json:"hls_url"`
	Name    string `json:"file_name,omitempty"`
}

type UploadVideoArchiveData struct {
	BatchID      string               `json:"batch_id,omitempty"`
	Total        int                  `json:"total"`
	Uploaded     int                  `json:"uploaded"`
	Failed       int                  `json:"failed"`
	Skipped      int                  `json:"skipped"`
	Videos       []UploadVideoData    `json:"videos"`
	Errors       []UploadArchiveError `json:"errors,omitempty"`
	SkippedFiles []string             `json:"skipped_files,omitempty"`
}

type UploadArchiveError struct {
	FileName string `json:"file_name"`
	Error    string `json:"error"`
}

type ArchiveProcessingProgressData struct {
	BatchID    string `json:"batch_id"`
	Total      int    `json:"total"`
	Transcoded int    `json:"transcoded"`
	Vectorized int    `json:"vectorized"`
}

type UploadCoverData struct {
	VideoID  uint64 `json:"video_id"`
	CoverURL string `json:"cover_url"`
}

type InitiateChunkedUploadRequest struct {
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	UserID      uint64 `json:"user_id"`
	FileSize    int64  `json:"file_size"`
	ChunkSize   int64  `json:"chunk_size"`
	TotalChunks int    `json:"total_chunks"`
}

type ChunkedUploadData struct {
	UploadID       string `json:"upload_id"`
	FileName       string `json:"file_name"`
	FileSize       int64  `json:"file_size"`
	ChunkSize      int64  `json:"chunk_size"`
	TotalChunks    int    `json:"total_chunks"`
	UploadedChunks []int  `json:"uploaded_chunks"`
	Completed      bool   `json:"completed"`
}

type UploadVideoResponse struct {
	Success bool            `json:"success"`
	Data    UploadVideoData `json:"data"`
}

type UploadVideoArchiveResponse struct {
	Success bool                   `json:"success"`
	Data    UploadVideoArchiveData `json:"data"`
}

type ArchiveProcessingProgressResponse struct {
	Success bool                          `json:"success"`
	Data    ArchiveProcessingProgressData `json:"data"`
}

type UploadCoverResponse struct {
	Success bool            `json:"success"`
	Data    UploadCoverData `json:"data"`
}

type ChunkedUploadResponse struct {
	Success bool              `json:"success"`
	Data    ChunkedUploadData `json:"data"`
}
