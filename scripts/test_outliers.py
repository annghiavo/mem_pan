import json
import pandas as pd
from datetime import datetime, timedelta
from itertools import accumulate

def cum_concat(x):
    return list(accumulate(x))

def remove_outliers(group: pd.DataFrame) -> pd.DataFrame:
    print("Group columns in remove_outliers:", group.columns.tolist())
    print("Group name:", getattr(group, "name", "None"))
    # If first_rating is not in columns, add it back from group.name
    if "first_rating" not in group.columns:
        group = group.copy()
        group["first_rating"] = getattr(group, "name", "3")
        
    grouped_group = (
        group.groupby(by=["first_rating", "delta_t"], group_keys=False)
        .agg({"y": ["mean", "count"]})
        .reset_index()
    )
    sort_index = grouped_group.sort_values(
        by=[("y", "count"), "delta_t"], ascending=[True, False]
    ).index

    total = sum(grouped_group[("y", "count")])
    has_been_removed = 0
    for i in sort_index:
        count = grouped_group.loc[i, ("y", "count")].values[0] if isinstance(grouped_group.loc[i, ("y", "count")], pd.Series) else grouped_group.loc[i, ("y", "count")]
        delta_t = grouped_group.loc[i, "delta_t"]
        if isinstance(delta_t, pd.Series):
            delta_t = delta_t.values[0]
            
        if has_been_removed + count >= max(total * 0.05, 20):
            if count < 6 or delta_t > (100 if group.name[0] != "4" else 365):
                group.drop(group[group["delta_t"] == delta_t].index, inplace=True)
                has_been_removed += count
        else:
            group.drop(group[group["delta_t"] == delta_t].index, inplace=True)
            has_been_removed += count
    return group

def remove_non_continuous_rows(group):
    discontinuity = group["i"].diff().fillna(1).ne(1)
    if not discontinuity.any():
        return group
    else:
        first_non_continuous_index = discontinuity.idxmax()
        return group.loc[: first_non_continuous_index - 1]

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
    
    # Calculate delta_t as float days:
    df["delta_t"] = df["review_time"].diff().fillna(0) / 1000 / 86400
    df.fillna({"delta_t": 0}, inplace=True)
    df["i"] = df.groupby("card_id").cumcount() + 1
    df.loc[df["i"] == 1, "delta_t"] = -1
    
    r_history_list = df.groupby("card_id", group_keys=False)["review_rating"].apply(
        lambda x: cum_concat([[i] for i in x])
    )
    df["r_history"] = [
        ",".join(map(str, item[:-1]))
        for sublist in r_history_list
        for item in sublist
    ]
    
    df = df[
        (df["review_rating"] != 0)
        & (df["r_history"].str.contains("0") == 0)
        & (df["delta_t"] != 0)
    ].copy()
    
    df["i"] = df.groupby("card_id").cumcount() + 1
    df["first_rating"] = df["r_history"].map(lambda x: x[0] if len(x) > 0 else "")
    df["y"] = df["review_rating"].map(lambda x: {1: 0, 2: 1, 3: 1, 4: 1}[x])
    
    print(f"Shape: {df.shape}")
    print(f"Final eligible (i > 1 and delta_t > 0): {len(df[(df['i'] > 1) & (df['delta_t'] > 0)])}")

if __name__ == "__main__":
    run_test()
