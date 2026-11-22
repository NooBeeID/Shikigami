# Spike monitor aktivitas — payload hook dan berkas Claude

Dijalankan 2026-11-22 dengan `claude -p` (Claude Code 2.1.330) dan hook pencatat via `--settings`. Satu `Agent` (foreground) dan satu `Workflow` dua agent dengan `args {ticket: "docs/tickets/T-SPIKE.md"}`. Fixture ada di `internal/runs/testdata/` (path pribadi diganti `/Users/USER`).

## Jawaban

**(a) Sub agent.** Nama tool `Agent`. `PreToolUse.tool_input` = `{description, prompt, run_in_background}`; `tool_use_id` ada. Identitas = `description`.

**(b) Korelasi sub agent.** `SubagentStart` hanya membawa `agent_id` dan `agent_type` (`general-purpose` untuk Agent biasa, `workflow-subagent` untuk agent milik workflow); TIDAK ada `tool_use_id`. Korelasi tersedia lewat dua jalur: `PostToolUse(Agent).tool_response.agentId` dan berkas `<sesi>/subagents/agent-<agent_id>.meta.json` yang memuat `toolUseId`, `description`, `finished`. `SubagentStop` memuat `agent_id`, `agent_transcript_path`, `last_assistant_message`.

**(c) Workflow.** `PreToolUse.tool_input` = `{script, args}` (tanpa `name`). `PostToolUse.tool_response` = `{status:"async_launched", taskId, taskType:"local_workflow", workflowName, runId, summary, transcriptDir, scriptPath}` — jadi `PostToolUse` Workflow adalah PELUNCURAN, dan sumber `workflowName`/`runId`/`taskId`. `workflows/wf_<runId>.json` BARU ditulis saat workflow selesai (mtime = waktu selesai), jadi TIDAK ada selama berjalan. Namun `subagents/workflows/<runId>/journal.jsonl` ditulis bertahap selama berjalan: baris `{"type":"launched"}`, `{"type":"started", agentId, label, phase}`, `{"type":"result", agentId, result}`. Itu sumber progres live (fase berjalan, jumlah agent mulai/selesai, label). Agent milik workflow: `subagents/workflows/<runId>/agent-<id>.meta.json` (`agentType:"workflow-subagent"`, `description`=label, `workflowPhase`, `inWorkflowRun`).

**(d) Penanda selesai workflow.** Hook `Stop` dan `SubagentStop` memuat `background_tasks`: daftar `{id, type, status, description, name}` untuk tugas latar belakang yang masih berjalan; `id` = `taskId` dari `PostToolUse`. Workflow selesai saat `taskId`-nya tidak lagi ada di `background_tasks` pada `Stop` berikutnya (`[]` saat tidak ada). Cadangan: `wf_<runId>.json` dengan `status: "completed"`.

**Checklist sub agent.** Tidak terverifikasi: agent probe tidak memakai TodoWrite/TaskCreate. Bentuk transkrip `.jsonl` agent terbukti (baris `user`/`assistant`/`attachment`, bidang `agentId`); pembacaan checklist adalah upaya terbaik dan harus fail-soft.

## Keputusan cabang (mengubah rencana)
1. Workflow dibuat di `PreToolUse` (kunci `tool_use_id`, identitas dari `args`), dilengkapi di `PostToolUse` (`taskId`, `workflowName`, `runId`, `transcriptDir`). Berakhir saat hilang dari `background_tasks` di `Stop`/`SubagentStop` sesi itu.
2. Progres workflow dibaca dari `journal.jsonl` (bukan `wf_*.json`); `wf_<runId>.json` hanya cadangan untuk deteksi selesai.
3. Sub agent biasa dibuat di `SubagentStart` (kunci `agent_id`, abaikan `workflow-subagent`); identitas diambil dari `description` di `agent-<id>.meta.json` (`toolUseId` memuat korelasi ke `PreToolUse`), jatuh ke `PreToolUse` yang belum diklaim bila berkas belum ada. Berakhir di `SubagentStop`.
4. `Agent` dengan `run_in_background: true` belum diuji; diperlakukan seperti foreground (berakhir di `SubagentStop`).
