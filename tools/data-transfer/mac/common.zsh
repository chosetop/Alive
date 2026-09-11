#!/bin/zsh

set -euo pipefail

TRANSFER_ROOT="${ALIVE_TRANSFER_ROOT:-$HOME/Documents/Alive数据交换}"
RUNTIME_ENV="$HOME/Library/Application Support/Alive/backend/.env"
REPO_ENV="${0:A:h:h:h:h}/backend/.env"

die() {
  echo "错误：$*" >&2
  exit 1
}

pause() {
  if [[ -t 0 ]]; then
    read -r "?按回车关闭窗口..."
  fi
}

find_pg_tool() {
  local name="$1"
  local candidate
  candidate="$(command -v "$name" 2>/dev/null || true)"
  if [[ -n "$candidate" ]]; then
    echo "$candidate"
    return
  fi
  for candidate in /opt/homebrew/opt/postgresql@18/bin/$name /opt/homebrew/opt/libpq/bin/$name /usr/local/bin/$name; do
    [[ -x "$candidate" ]] && { echo "$candidate"; return; }
  done
  die "找不到 $name。请先执行：brew install postgresql@18"
}

load_database_url() {
  local env_file=""
  local line=""
  if [[ -f "$RUNTIME_ENV" ]]; then
    env_file="$RUNTIME_ENV"
  elif [[ -f "$REPO_ENV" ]]; then
    env_file="$REPO_ENV"
  fi
  if [[ -n "$env_file" ]]; then
    line="$(grep -m1 '^DATABASE_URL=' "$env_file" 2>/dev/null || true)"
  fi
  DATABASE_URL="${ALIVE_DATABASE_URL:-${line#DATABASE_URL=}}"
  [[ -n "$DATABASE_URL" ]] || DATABASE_URL="alive"
  export DATABASE_URL
}

ensure_transfer_dirs() {
  mkdir -p "$TRANSFER_ROOT/待导入" "$TRANSFER_ROOT/已导入" \
    "$TRANSFER_ROOT/手动备份" "$TRANSFER_ROOT/导入前自动备份"
  chmod 700 "$TRANSFER_ROOT" "$TRANSFER_ROOT"/*
}

sha256_file() {
  /usr/bin/shasum -a 256 "$1" | /usr/bin/awk '{print $1}'
}

sql_scalar() {
  local psql_bin="$1"
  local sql="$2"
  "$psql_bin" "$DATABASE_URL" -X -qAt -v ON_ERROR_STOP=1 -c "$sql"
}

stop_mac_backend() {
  BACKEND_WAS_LOADED=0
  if /bin/launchctl print "gui/$(id -u)/com.alive.backend" >/dev/null 2>&1; then
    BACKEND_WAS_LOADED=1
    /bin/launchctl bootout "gui/$(id -u)/com.alive.backend" >/dev/null 2>&1 || true
  fi
}

restart_mac_backend() {
  if [[ "${BACKEND_WAS_LOADED:-0}" == "1" ]]; then
    local plist="$HOME/Library/LaunchAgents/com.alive.backend.plist"
    [[ -f "$plist" ]] && /bin/launchctl bootstrap "gui/$(id -u)" "$plist" >/dev/null 2>&1 || true
  fi
}
