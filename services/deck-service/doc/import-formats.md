# Import File Formats

The import endpoint (`POST /v1/import/parse`) accepts three file types. Choose based on where your cards are coming from.

---

## CSV / TSV

**Best for:** spreadsheets, Anki exports, any tool that can export rows.

Each row is one card. The first column is the **front**, the second is the **back**. Extra columns are ignored. The separator is detected automatically — use commas (`.csv`) or tabs (`.tsv`).

```
front,back
Abide by,To follow set rules or norms
Allude to,To mention something indirectly
```

```
Abide by	To follow set rules or norms
Allude to	To mention something indirectly
```

**Rules:**
- No header row required; if a header is present, it becomes the first card unless you remove it.
- Rows where front or back is blank are skipped.
- UTF-8 with or without BOM is accepted.

---

## PDF

**Best for:** Quizlet PDF exports.

The file must be a two-column table where each row starts with a number:

| # | Term | Definition |
|---|------|------------|
| 1. | Abide by | To follow set rules or norms |
| 2. | Allude to | To mention something indirectly |

Export from Quizlet: open a set → **⋯ More** → **Export** → choose **PDF**.

**Rules:**
- Scanned or image-based PDFs are not supported (text must be selectable).
- Only the numbered rows are extracted; headers and footers are ignored.
- Each row must have exactly one term and one definition.

---

## Choosing a format

| Situation | Use |
|-----------|-----|
| Exporting from a spreadsheet | CSV or TSV |
| Exporting from Anki | TSV (Anki's default export) |
| Exporting from Quizlet | PDF |
| Copying from a text editor | CSV |
