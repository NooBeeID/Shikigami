package runs

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// maxReadBytes membatasi byte yang dibaca dari journal/transkrip (bagian ekor).
const maxReadBytes = 4 << 20

// Detail adalah hasil membaca berkas Claude untuk satu run. Zero value berarti
// "tidak diketahui" (berkas tak ada, tak terbaca, atau rusak).
type Detail struct {
	Found, Finished bool
	Progress        *Progress
	// Description hanya untuk sub agent: description dari agent-<id>.meta.json,
	// sumber identitas yang pasti (FIFO pending bisa tertukar).
	Description string
}

// safeID menolak ID kosong atau yang bisa dipakai untuk keluar dari folder.
func safeID(id string) bool {
	return id != "" && id != "." && !strings.ContainsAny(id, `/\`) && !strings.Contains(id, "..") && !strings.ContainsRune(id, 0)
}

// readTailLines membaca paling banyak maxReadBytes dari ekor berkas dan
// memecahnya per baris. Baris pertama dibuang bila pembacaan dipotong di
// tengah baris.
func readTailLines(path string) ([]string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		return nil, false
	}
	truncated := st.Size() > maxReadBytes
	if truncated {
		if _, err := f.Seek(st.Size()-maxReadBytes, io.SeekStart); err != nil {
			return nil, false
		}
	}
	data, err := io.ReadAll(io.LimitReader(f, maxReadBytes))
	if err != nil {
		return nil, false
	}
	parts := bytes.Split(data, []byte("\n"))
	if truncated && len(parts) > 0 {
		parts = parts[1:]
	}
	lines := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(string(p)); s != "" {
			lines = append(lines, s)
		}
	}
	return lines, true
}

// readSmallJSON membaca berkas JSON kecil (dibatasi maxReadBytes).
func readSmallJSON(path string, v any) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxReadBytes))
	if err != nil {
		return false
	}
	return json.Unmarshal(data, v) == nil
}

// ReadDetail membaca progres run dari berkas sesi Claude. Tidak pernah error
// atau panic; hasil tak lengkap dikembalikan sebagai Detail parsial/kosong.
func ReadDetail(transcriptPath string, run Run) (d Detail) {
	defer func() {
		if recover() != nil {
			d = Detail{}
		}
	}()
	if transcriptPath == "" {
		return Detail{}
	}
	folder := strings.TrimSuffix(transcriptPath, ".jsonl")
	switch run.Kind {
	case KindWorkflow:
		if safeID(run.RunID) {
			return readWorkflow(folder, run.RunID)
		}
	case KindSubagent:
		if safeID(run.AgentID) {
			return readSubagent(folder, run.AgentID)
		}
	}
	return Detail{}
}

// runID asli sudah berawalan "wf_" (sama dengan nama berkas di workflows/).
func readWorkflow(folder, runID string) Detail {
	var d Detail
	var wf struct {
		Status string `json:"status"`
	}
	if readSmallJSON(filepath.Join(folder, "workflows", runID+".json"), &wf) {
		s := strings.ToLower(wf.Status)
		d.Finished = s != "" && s != "running"
	}
	lines, ok := readTailLines(filepath.Join(folder, "subagents", "workflows", runID, "journal.jsonl"))
	if !ok {
		return d
	}
	type entry struct {
		Type    string `json:"type"`
		AgentID string `json:"agentId"`
		Label   string `json:"label"`
		Phase   string `json:"phase"`
	}
	p := &Progress{}
	idx := map[string]int{}
	results := map[string]bool{}
	valid := 0
	for _, ln := range lines {
		var e entry
		if json.Unmarshal([]byte(ln), &e) != nil {
			continue // baris rusak atau baris terakhir yang masih ditulis
		}
		valid++
		switch e.Type {
		case "started":
			if _, dup := idx[e.AgentID]; dup {
				continue
			}
			idx[e.AgentID] = len(p.Agents)
			p.Agents = append(p.Agents, AgentLine{Label: e.Label, Phase: e.Phase})
			p.PhaseTitle = e.Phase
		case "result":
			results[e.AgentID] = true
		}
	}
	if valid == 0 && len(lines) > 0 {
		return Detail{Finished: d.Finished}
	}
	for id, i := range idx {
		if results[id] {
			p.Agents[i].Done = true
			p.Done++
		}
	}
	p.Total = len(p.Agents)
	d.Found = true
	d.Progress = p
	return d
}

func readSubagent(folder, agentID string) Detail {
	base := filepath.Join(folder, "subagents", "agent-"+agentID)
	var meta struct {
		Finished    bool   `json:"finished"`
		Description string `json:"description"`
	}
	if !readSmallJSON(base+".meta.json", &meta) {
		return Detail{}
	}
	d := Detail{Found: true, Finished: meta.Finished, Description: meta.Description}
	d.Progress = readChecklist(base + ".jsonl")
	return d
}

// readChecklist mengambil TodoWrite terakhir dari transkrip sub agent.
// Format TIDAK terverifikasi terhadap keluaran Claude asli (upaya terbaik).
func readChecklist(path string) *Progress {
	lines, ok := readTailLines(path)
	if !ok {
		return nil
	}
	for i := len(lines) - 1; i >= 0; i-- {
		var e struct {
			Type    string `json:"type"`
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal([]byte(lines[i]), &e) != nil || e.Type != "assistant" {
			continue
		}
		var blocks []struct {
			Type  string `json:"type"`
			Name  string `json:"name"`
			Input struct {
				Todos []struct {
					Status string `json:"status"`
				} `json:"todos"`
			} `json:"input"`
		}
		if json.Unmarshal(e.Message.Content, &blocks) != nil {
			continue
		}
		for j := len(blocks) - 1; j >= 0; j-- {
			b := blocks[j]
			if b.Type != "tool_use" || b.Name != "TodoWrite" {
				continue
			}
			p := &Progress{Total: len(b.Input.Todos)}
			for _, t := range b.Input.Todos {
				if t.Status == "completed" {
					p.Done++
				}
			}
			return p
		}
	}
	return nil
}
