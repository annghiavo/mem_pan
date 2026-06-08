import psycopg2
import pandas as pd
from fsrs_optimizer import Optimizer  # type: ignore

study_db_uri = "postgresql://neondb_owner:npg_u4lhpDfRgj6E@ep-flat-cloud-ao5gwidz-pooler.c-2.ap-southeast-1.aws.neon.tech/neondb?sslmode=require"
user_id = "c12adea4-2dc6-4303-be27-4ab1896bb8c8"

def test_pipeline():
    conn = psycopg2.connect(study_db_uri)
    cursor = conn.cursor()
    cursor.execute("""
        SELECT card_id, rating, elapsed_days, review_time
        FROM revlogs
        WHERE user_id = %s
        ORDER BY review_time ASC
    """, (user_id,))
    
    rows = cursor.fetchall()
    print(f"Fetched {len(rows)} reviews from db.")
    
    data = []
    for r in rows:
        # r[3] is a datetime object. Convert to unix seconds:
        review_time_s = int(r[3].timestamp())
        data.append({
            "review_time": review_time_s * 1000,
            "card_id": str(r[0]),
            "review_rating": int(r[1]),
            "review_duration": 6000,
            "review_state": 1,
        })
        
    df_raw = pd.DataFrame(data)
    df_raw.to_csv("revlog.csv", index=False)
    
    optimizer = Optimizer()
    optimizer.create_time_series("UTC", "2006-10-02", 4, analysis=False)
    
    # Let's inspect the created revlog_history.tsv
    df_hist = pd.read_csv("./revlog_history.tsv", sep="\t")
    print(f"revlog_history.tsv has {len(df_hist)} rows.")
    print("Columns:", df_hist.columns.tolist())
    
    print("\nFirst 10 rows of history:")
    print(df_hist.head(10))
    
    # Filter as pretrain does:
    df_filtered = df_hist[(df_hist["i"] > 1) & (df_hist["delta_t"] > 0)]
    print(f"\nFiltered dataset has {len(df_filtered)} rows.")
    if len(df_filtered) > 0:
        print(df_filtered.head(10))
        
    # Let's see what happens to S0_dataset_group
    print("\nS0_dataset_group:")
    print(optimizer.S0_dataset_group)

if __name__ == "__main__":
    test_pipeline()
