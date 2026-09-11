#!/bin/zsh

SCRIPT_DIR="${0:A:h}"
source "$SCRIPT_DIR/common.zsh"
trap pause EXIT

echo "== 导入 Alive 数据 =="
ensure_transfer_dirs
load_database_url

PG_DUMP="$(find_pg_tool pg_dump)"
PG_RESTORE="$(find_pg_tool pg_restore)"
PSQL="$(find_pg_tool psql)"
PACKAGE="${1:-}"
if [[ -z "$PACKAGE" ]]; then
  PACKAGE="$(osascript -e 'POSIX path of (choose file with prompt "选择 Alive 数据交换包" of type {"public.zip-archive"})' 2>/dev/null || true)"
fi
[[ -f "$PACKAGE" ]] || die "没有选择有效的数据交换包。"

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/alive-import.XXXXXX")"
trap 'restart_mac_backend; rm -rf "$WORK_DIR"; pause' EXIT
/usr/bin/ditto -x -k "$PACKAGE" "$WORK_DIR"
DUMP_FILE="$(find "$WORK_DIR" -type f -name alive.dump -print -quit)"
[[ -n "$DUMP_FILE" ]] || die "交换包中没有 alive.dump。"
PACKAGE_DIR="${DUMP_FILE:h}"

EXPECTED="$(awk '/alive\.dump/{print $1; exit}' "$PACKAGE_DIR/SHA256.txt" 2>/dev/null || true)"
ACTUAL="$(sha256_file "$DUMP_FILE")"
[[ -n "$EXPECTED" && "$EXPECTED" == "$ACTUAL" ]] || die "备份校验失败，已停止导入。"
"$PG_RESTORE" --list "$DUMP_FILE" >/dev/null || die "无法读取数据库备份。"

echo "即将用以下交换包覆盖本机 Alive 数据库："
cat "$PACKAGE_DIR/manifest.json" 2>/dev/null || true
read -r "CONFIRM?输入 IMPORT 确认导入："
[[ "$CONFIRM" == "IMPORT" ]] || die "已取消，数据库未修改。"

STAMP="$(date +%Y%m%d-%H%M%S)"
SAFETY="$TRANSFER_ROOT/导入前自动备份/alive-before-import-$STAMP.dump"
echo "正在创建导入前安全备份..."
"$PG_DUMP" --format=custom --no-owner --no-privileges --file="$SAFETY" "$DATABASE_URL"
"$PG_RESTORE" --list "$SAFETY" >/dev/null
chmod 600 "$SAFETY"

stop_mac_backend
echo "正在恢复数据库..."
"$PSQL" "$DATABASE_URL" -X -v ON_ERROR_STOP=1 -c "select pg_terminate_backend(pid) from pg_stat_activity where datname=current_database() and pid<>pg_backend_pid();" >/dev/null
"$PG_RESTORE" --clean --if-exists --no-owner --no-privileges --exit-on-error --dbname="$DATABASE_URL" "$DUMP_FILE"

SCHEMA_VERSION="$(sql_scalar "$PSQL" "select version::text || case when dirty then '-dirty' else '' end from schema_migrations limit 1;")"
ENTRY_COUNT="$(sql_scalar "$PSQL" "select count(*) from entries where deleted_at is null;")"
restart_mac_backend
BACKEND_WAS_LOADED=0

mkdir -p "$TRANSFER_ROOT/已导入"
cp -p "$PACKAGE" "$TRANSFER_ROOT/已导入/$(basename "$PACKAGE")"
echo "导入成功。有效内容数：$ENTRY_COUNT；数据库迁移版本：$SCHEMA_VERSION"
echo "导入前备份：$SAFETY"
