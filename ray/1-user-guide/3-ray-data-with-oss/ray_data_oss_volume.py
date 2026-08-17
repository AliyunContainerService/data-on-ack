import os

import pandas as pd
import ray

ray.init(address="auto")

csv_path = "/home/ray/iris.csv"
output_dir = "/mnt/oss/volume_iris_output"

# 1. Read: load the Iris CSV baked into the image
df = pd.read_csv(csv_path)
ds = ray.data.from_pandas(df)
print(f"Loaded {ds.count()} rows from {csv_path}")

# 2. Write: store the dataset as parquet on the OSS volume mount
#    (read/write via local filesystem — no S3FileSystem needed)
#    Write only once: if parquet files already exist, skip and read them directly
if os.path.exists(output_dir) and any(f.endswith(".parquet") for f in os.listdir(output_dir)):
    print(f"Parquet output already exists at {output_dir}, skip writing")
else:
    ds.write_parquet(output_dir)
    print(f"Wrote parquet to {output_dir}")

# 3. Read back the parquet from the OSS volume mount
ds2 = ray.data.read_parquet(output_dir)
print(f"Read back {ds2.count()} rows from {output_dir}")

# 4. Filter + group-by aggregation: mean of each numeric column per species
filtered = ds2.filter(lambda row: row["sepal_length"] > 5.0)
print(f"Rows with sepal_length > 5.0: {filtered.count()}")
result = filtered.groupby("species").mean()
print(result.to_pandas().to_string(index=False))