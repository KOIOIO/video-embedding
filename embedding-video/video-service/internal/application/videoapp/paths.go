package videoapp

// Paths 保存本地路径与对外 URL 前缀之间的映射关系。
type Paths struct {
	RawDir          string
	HLSDir          string
	RawObjectPrefix string
	HLSObjectPrefix string
	RawURLPrefix    string
	HLSURLPrefix    string
	CoverURLPrefix  string
	HLSMasterName   string
}

// UploadPlan 描述上传开始时计算出的所有路径、对象键与回传 URL。
type UploadPlan struct {
	OriginalFileName string
	StoredFileName   string
	DatePath         string
	RawAbsPath       string
	RawObjectKey     string
	RawURL           string
	HLSAbsDir        string
	HLSObjectPrefix  string
	HLSURL           string
	RawUploaded      bool
}

// UploadResult 是上传流程完成后返回给协议层的摘要信息。
type UploadResult struct {
	VideoID uint64
	TaskID  string
	RawURL  string
	HLSURL  string
	Name    string
}

type ArchiveUploadResult struct {
	Total    int
	Uploaded []UploadResult
	Failed   []ArchiveUploadFailure
	Skipped  []string
}

type ArchiveUploadFailure struct {
	FileName string
	Error    string
}
