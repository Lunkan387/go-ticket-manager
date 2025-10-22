#!/usr/bin/env python3
import sqlite3, sys, re, pathlib, argparse

def get_tables(conn):
    cur = conn.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
    return [r[0] for r in cur.fetchall()]

def get_columns(conn, table):
    cur = conn.execute(f"PRAGMA table_info('{table}')")
    # cid, name, type, notnull, dflt_value, pk
    cols = []
    for cid, name, typ, notnull, dflt, pk in cur.fetchall():
        cols.append({"name": name, "type": typ or "TEXT", "pk": bool(pk)})
    return cols

def get_foreign_keys(conn, table):
    cur = conn.execute(f"PRAGMA foreign_key_list('{table}')")
    # id, seq, table, from, to, on_update, on_delete, match
    fks = []
    for _, _, ref_table, col_from, col_to, *_ in cur.fetchall():
        fks.append({"from_table": table, "from_col": col_from,
                    "to_table": ref_table, "to_col": col_to or "id"})
    return fks

def guess_foreign_keys(table, cols, tables):
    guesses = []
    for c in cols:
        m = re.fullmatch(r"(.+)_id", c["name"])
        if m:
            cand = m.group(1)
            # try exact, plural, singular
            candidates = [cand, cand + "s", cand.rstrip('s')]
            for t in candidates:
                if t in tables and t != table:
                    guesses.append({"from_table": table, "from_col": c["name"],
                                    "to_table": t, "to_col": "id", "guessed": True})
                    break
    return guesses

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("db", help="SQLite database path")
    ap.add_argument("--guess", action="store_true", help="Deviner les relations par nom (xxx_id)")
    ap.add_argument("--schema", default=None, help="Limiter aux tables (liste séparée par des virgules)")
    args = ap.parse_args()

    dbpath = pathlib.Path(args.db)
    if not dbpath.exists():
        sys.exit(f"Base introuvable: {dbpath}")

    conn = sqlite3.connect(str(dbpath))
    conn.execute("PRAGMA foreign_keys=ON")

    tables = get_tables(conn)
    if args.schema:
        wanted = set([t.strip() for t in args.schema.split(",") if t.strip()])
        tables = [t for t in tables if t in wanted]

    # Collecte colonnes et FKs
    table_cols = {t: get_columns(conn, t) for t in tables}
    fks = []
    for t in tables:
        fks.extend([fk for fk in get_foreign_keys(conn, t)
                    if fk["to_table"] in tables])

    if args.guess:
        for t in tables:
            fks.extend(guess_foreign_keys(t, table_cols[t], set(tables)))

    # Début Mermaid
    out = []
    out.append("erDiagram")
    # Entités
    for t in tables:
        out.append(f"  {t} {{")
        for c in table_cols[t]:
            # petites annotations PK/FK dans le type pour lisibilité
            tag = " PK" if c["pk"] else ""
            out.append(f"    {c['type']}{tag} {c['name']}")
        out.append("  }")

    # Relations (parent ||--o{ child)
    # from_table (child) -> to_table (parent)
    seen = set()
    for fk in fks:
        parent = fk["to_table"]
        child = fk["from_table"]
        label = f"{child}.{fk['from_col']}→{parent}.{fk['to_col']}"
        key = (parent, child, label)
        if key in seen: 
            continue
        seen.add(key)
        arrow = "||--o{"  # 1 parent vers N enfants
        suffix = " : " + ("FK" if "guessed" not in fk else "guessed")
        out.append(f"  {parent} {arrow} {child}{suffix} \"{label}\"")

    print("\n".join(out))

if __name__ == "__main__":
    main()

