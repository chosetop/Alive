#!/bin/zsh

SCRIPT_DIR="${0:A:h}"
source "$SCRIPT_DIR/common.zsh"
trap pause EXIT

echo "== 导出 Alive 数据 =="
ensure_transfer_dirs
load_database_url

PG_DUMP="$(find_pg_tool pg_dump)"
PG_RESTORE="$(find_pg_tool pg_restore)"
PSQL="$(find_pg_tool psql)"
STAMP="$(date +%Y%m%d-%H%M%S)"
PACKAGE_NAME="Alive-$STAMP"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/alive-export.XXXXXX")"
PACKAGE_DIR="$WORK_DIR/$PACKAGE_NAME"
OUTPUT="$TRANSFER_ROOT/手动备份/$PACKAGE_NAME.zip"
mkdir -p "$PACKAGE_DIR"
trap 'rm -rf "$WORK_DIR"; pause' EXIT

echo "正在创建一致性数据库快照..."
"$PG_DUMP" --format=custom --no-owner --no-privileges --file="$PACKAGE_DIR/alive.dump" "$DATABASE_URL"
"$PG_RESTORE" --list "$PACKAGE_DIR/alive.dump" >/dev/null

DB_VERSION="$(sql_scalar "$PSQL" "show server_version;")"
SCHEMA_VERSION="$(sql_scalar "$PSQL" "select version::text || case when dirty then '-dirty' else '' end from schema_migrations limit 1;" 2>/dev/null || echo unknown)"
ENTRY_COUNT="$(sql_scalar "$PSQL" "select count(*) from entries where deleted_at is null;" 2>/dev/null || echo 0)"
CHECKSUM="$(sha256_file "$PACKAGE_DIR/alive.dump")"
HOST_NAME="$(scutil --get ComputerName 2>/dev/null || hostname)"

cat > "$PACKAGE_DIR/manifest.json" <<EOF
{
  "format": 1,
  "source_os": "macOS",
  "source_computer": "$HOST_NAME",
  "exported_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "postgres_version": "$DB_VERSION",
  "schema_version": "$SCHEMA_VERSION",
  "active_entries": $ENTRY_COUNT,
  "dump_sha256": "$CHECKSUM"
}
EOF
echo "$CHECKSUM  alive.dump" > "$PACKAGE_DIR/SHA256.txt"
cat > "$PACKAGE_DIR/README.txt" <<EOF
Alive PostgreSQL 数据交换包
导出时间：$(date '+%Y-%m-%d %H:%M:%S %z')
请使用 Alive 项目内的一键导入脚本恢复，不要手工解压后覆盖数据库目录。
EOF

rm -f "$OUTPUT"
/usr/bin/ditto -c -k --keepParent "$PACKAGE_DIR" "$OUTPUT"
chmod 600 "$OUTPUT"
echo "导出成功：$OUTPUT"
echo "有效内容数：$ENTRY_COUNT；数据库迁移版本：$SCHEMA_VERSION"
open -R "$OUTPUT" 2>/dev/null || true
