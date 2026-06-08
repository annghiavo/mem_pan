import json
import pandas as pd
from datetime import datetime, timedelta

def remove_non_continuous_rows(group):
    discontinuity = group["i"].diff().fillna(1).ne(1)
    if not discontinuity.any():
        return group
    else:
        first_non_continuous_index = discontinuity.idxmax()
        return group.loc[: first_non_continuous_index - 1]

def remove_outliers(group):
    # FSRS outliers removal logic:
    # It removes rows where delta_t is far from the median.
    # Let's see if we can find its definition in fsrs_optimizer.py
    # Since we don't have it, let's just see if we can log its effect
    # We can mock/check it. Let's see:
    q1 = group["delta_t"].quantile(0.25)
    q3 = group["delta_t"].quantile(0.75)
    iqr = q3 - q1
    return group[(group["delta_t"] >= q1 - 1.5 * iqr) & (group["delta_t"] <= q3 + 1.5 * iqr)]

def run_test():
    with open("scripts/revlogs.json", "r") as f:
        logs = json.load(f)
        
    data = []
    for r in logs:
        # review_time is serialized as RFC3339 in json: "2026-05-21T16:02:39.584846Z"
        # Parse and convert to unix milliseconds
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
    print(f"Initial df shape: {df.shape}")
    
    df.sort_values(by=["card_id", "review_time"], inplace=True, ignore_index=True)
    df["review_date"] = pd.to_datetime(df["review_time"] // 1000, unit="s")
    df["review_date"] = df["review_date"].dt.tz_localize("UTC") # hardcoded timezone "UTC" in our _run_optimizer
    
    df["real_days"] = df["review_date"] - timedelta(hours=4)
    df["real_days"] = pd.DatetimeIndex(
        df["real_days"].dt.floor("D")
    ).to_julian_date()
    
    # Calculate delta_t
    df["delta_t"] = df.real_days.diff()
    df.fillna({"delta_t": 0}, inplace=True)
    
    df["i"] = df.groupby("card_id").cumcount() + 1
    df.loc[df["i"] == 1, "delta_t"] = -1
    
    print(f"After initial create_time_series prep, shape: {df.shape}")
    
    # Simulate first_rating and last_rating computation:
    # ... (skipping simulation config extraction)
    # Let's mock:
    df["r_history"] = ""
    
    df = df[
        (df["review_rating"] != 0)
        & (df["delta_t"] != 0)
    ].copy()
    print(f"After (review_rating != 0) & (delta_t != 0) filter, shape: {df.shape}")
    
    df["i"] = df.groupby("card_id").cumcount() + 1
    print(f"Recalculating i, shape: {df.shape}")
    
    # Let's count how many have i == 2
    i_counts = df["i"].value_counts()
    print("i counts:")
    print(i_counts)
    
    df["first_rating"] = "3" # mock first rating
    
    # Outliers removal:
    df_i2 = df[df["i"] == 2]
    print(f"i == 2 count before outliers removal: {len(df_i2)}")
    
    # If we run remove_non_continuous_rows:
    df_cont = df.groupby("card_id", as_index=False, group_keys=False).apply(remove_non_continuous_rows)
    print(f"After remove_non_continuous_rows, shape: {df_cont.shape}")
    print("Cont i counts:")
    print(df_cont["i"].value_counts())

if __name__ == "__main__":
    run_test()
