import json
import pandas as pd
from datetime import datetime, timedelta
from itertools import accumulate

def cum_concat(x):
    return list(accumulate(x))

def run_test():
    with open("scripts/revlogs.json", "r") as f:
        logs = json.load(f)
        
    data = []
    for r in logs:
        dt = datetime.fromisoformat(r["review_time"].replace("Z", "+00:00"))
        review_time_ms = int(dt.timestamp() * 1000)
        data.append({
            "review_time": review_time_ms,
            "card_id": str(r["card_id"]),
            "review_rating": int(r["rating"]),
            "review_duration": 6000,
            "review_state": 1,
        })
        
    df = pd.DataFrame(data)
    df.sort_values(by=["card_id", "review_time"], inplace=True, ignore_index=True)
    df["review_date"] = pd.to_datetime(df["review_time"] // 1000, unit="s")
    df["review_date"] = df["review_date"].dt.tz_localize("UTC")
    
    df["real_days"] = df["review_date"] - timedelta(hours=4)
    df["real_days"] = pd.DatetimeIndex(
        df["real_days"].dt.floor("D")
    ).to_julian_date()
    
    df["delta_t"] = df.real_days.diff()
    df.fillna({"delta_t": 0}, inplace=True)
    df["i"] = df.groupby("card_id").cumcount() + 1
    df.loc[df["i"] == 1, "delta_t"] = -1
    
    # EXACT FSRS logic for r_history:
    r_history_list = df.groupby("card_id", group_keys=False)["review_rating"].apply(
        lambda x: cum_concat([[i] for i in x])
    )
    df["r_history"] = [
        ",".join(map(str, item[:-1]))
        for sublist in r_history_list
        for item in sublist
    ]
    
    print("Unique values of r_history:")
    print(df["r_history"].value_counts())
    
    # Let's filter:
    df_filtered = df[
        (df["review_rating"] != 0)
        & (df["r_history"].str.contains("0") == 0)
        & (df["delta_t"] != 0)
    ].copy()
    
    print(f"\nAfter filter, shape: {df_filtered.shape}")

if __name__ == "__main__":
    run_test()
