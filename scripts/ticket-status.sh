#!/usr/bin/env bash
# Menampilkan status semua tiket di docs/tickets/ dengan membaca:
#  - frontmatter `status:` dan `depends_on:` di file tiket
#  - ada tidaknya docs/plans/<tiket>/overview.md dan progress.md
#  - git worktree aktif (worktree/feat-<tiket>)
# Kolom DEPENDS menampilkan dependensi tiap tiket: NN:ok (terpenuhi) atau NN:BELUM.
# Hanya membaca; tidak mengubah apa pun. Tanpa dependency selain bash, git, sed, grep.
set -u

common="$(git rev-parse --path-format=absolute --git-common-dir 2>/dev/null || true)"
if [ -n "$common" ]; then root="$(dirname "$common")"; else root="$(pwd)"; fi
cd "$root" || exit 1

shopt -s nullglob
files=(docs/tickets/*-ticket-*.md)
if [ ${#files[@]} -eq 0 ]; then
  echo "Tidak ada tiket di docs/tickets/. Jalankan /create-ticket-from-spec dulu."
  exit 0
fi

fmval() { grep -m1 "^$1:" "$2" 2>/dev/null | sed -e "s/^$1: *//" -e 's/[[:space:]]*$//'; }
st_of() { fmval status "docs/tickets/$1.md"; }
short() { echo "$1" | sed -E 's/^.*-ticket-//'; }
num() { short "$1" | sed -E 's/-.*$//'; }
satisfied() { # tiket dianggap memenuhi dependensi bila implemented, atau in-progress dengan kode sudah ter-merge (ada progress.md)
  local s; s="$(st_of "$1")"
  [ "$s" = "implemented" ] && return 0
  [ "$s" = "in-progress" ] && [ -f "docs/plans/$1/progress.md" ] && return 0
  return 1
}

worktrees="$(git worktree list 2>/dev/null || true)"

fmt='%-34s %-12s %-6s %-9s %-26s %s\n'
printf "$fmt" "TIKET" "STATUS" "PLAN" "PROGRESS" "DEPENDS" "LANGKAH BERIKUTNYA"
printf "$fmt" "-----" "------" "----" "--------" "-------" "------------------"

n_done=0; n_prog=0; n_ready=0; n_noplan=0
for f in "${files[@]}"; do
  n="$(basename "$f" .md)"
  st="$(fmval status "$f")"; [ -z "$st" ] && st="?"
  plan="tidak"; [ -f "docs/plans/$n/overview.md" ] && plan="ada"
  prog="tidak"; [ -f "docs/plans/$n/progress.md" ] && prog="ada"

  deps_raw="$(fmval depends_on "$f" | sed -e 's/^\[//' -e 's/\]$//' -e 's/,/ /g')"
  blocked=""; depinfo=""
  for d in $deps_raw; do
    [ -f "docs/tickets/$d.md" ] || continue
    if satisfied "$d"; then
      depinfo="$depinfo $(num "$d"):ok"
    else
      depinfo="$depinfo $(num "$d"):BELUM"
      blocked="$blocked $(short "$d")"
    fi
  done
  depinfo="$(echo "$depinfo" | sed -e 's/^ *//')"; [ -z "$depinfo" ] && depinfo="-"

  if [ "$st" = "implemented" ]; then
    next="-"; n_done=$((n_done+1))
  elif echo "$worktrees" | grep -q "worktree/feat-$n"; then
    next="sedang dikerjakan (worktree aktif)"; n_prog=$((n_prog+1))
  elif [ "$plan" = "tidak" ]; then
    next="/plan-from-tiket $n"; n_noplan=$((n_noplan+1))
  elif [ "$st" = "in-progress" ] && [ "$prog" = "ada" ]; then
    next="langkah manual tertunda, lihat docs/plans/$n/progress.md"; n_prog=$((n_prog+1))
  elif [ -n "$blocked" ]; then
    next="tunggu tiket:${blocked}"; n_ready=$((n_ready+1))
  else
    next="/implement-feature $n"; n_ready=$((n_ready+1))
  fi
  printf "$fmt" "$(short "$n")" "$st" "$plan" "$prog" "$depinfo" "$next"
done

echo
echo "Ringkasan: selesai=$n_done, sedang/tertunda=$n_prog, siap atau menunggu dependensi=$n_ready, belum diplan=$n_noplan, total=${#files[@]}"
echo "DEPENDS: NN:ok = tiket NN sudah terpenuhi (implemented, atau kodenya ter-merge); NN:BELUM = belum."
