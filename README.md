# mem_pan

export PATH=$PATH:/home/anvo/go/bin && make migrateup

CSV/TSV — two-column rows, auto-detected separator, blank rows skipped, BOM-safe           
  - PDF — Quizlet-style numbered two-column tables, text-selectable only                       
  - A quick decision table at the bottom so users can pick without reading the whole thing     
                                                                                               
  The note about the header row becoming a card is worth calling out — it's a common surprise  
  with CSV imports.

  check docker disk usage:
  docker system df
  docker system prune -a -f