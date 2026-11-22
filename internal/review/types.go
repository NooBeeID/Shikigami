// Paket review menyimpan baseline file yang disentuh agent per sesi.
// Penyimpanan: <DataDir>/review/<sessionId>/baseline/<sha256(pathRelatif)>
// dan <DataDir>/review/<sessionId>/index.json.
package review

// Status adalah jenis perubahan sebuah file relatif terhadap baseline.
type Status string

const (
	StatusModified Status = "modified"
	StatusAdded    Status = "added"
	StatusDeleted  Status = "deleted"
)

const (
	maxBaselineBytes = 1 << 20  // di atas ini: tooLarge, tanpa salinan isi
	HashAbsent       = "absent" // expectedHash untuk file yang tidak ada (deleted)
	IPCMethodFiles   = "review.files"
	EventChanged     = "review.changed"
)

// FileEntry adalah satu baris daftar file berubah yang dikirim ke UI.
type FileEntry struct {
	Path      string `json:"path"`
	Status    Status `json:"status"`
	Additions int    `json:"additions"` // 0 sampai ticket-02
	Deletions int    `json:"deletions"` // 0 sampai ticket-02
	Reviewed  bool   `json:"reviewed"`
	External  bool   `json:"external"` // false sampai ticket-03
	Binary    bool   `json:"binary"`
	TooLarge  bool   `json:"tooLarge"`
}

// TouchedFile adalah entri index.json mentah. Nama field JSON adalah kontrak
// tetap (hanya boleh ditambah, tidak diganti nama).
type TouchedFile struct {
	Path           string `json:"path"` // relatif cwd, '/'
	ExistedBefore  bool   `json:"existedBefore"`
	BaselineSha256 string `json:"baselineSha256"` // "" bila existedBefore=false
	FirstTouchAt   int64  `json:"firstTouchAt"`   // unix ms
	Reviewed       bool   `json:"reviewed"`
	ExpectedHash   string `json:"expectedHash"` // sha256 hex isi pasca-edit agent terakhir, atau HashAbsent
	Binary         bool   `json:"binary"`
	TooLarge       bool   `json:"tooLarge"`
}

// FilesResult adalah hasil IPC review.files.
type FilesResult struct {
	Supported bool        `json:"supported"`
	Files     []FileEntry `json:"files"` // selalu [] (bukan null)
}
