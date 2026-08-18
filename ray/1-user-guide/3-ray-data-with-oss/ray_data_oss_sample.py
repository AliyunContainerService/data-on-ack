import os

import pandas as pd
import pyarrow.fs as pfs
import ray

ray.init(address="auto")

endpoint = os.environ["OSS_ENDPOINT"]
bucket_name = os.environ["OSS_BUCKET"]
access_key_id = os.environ["OSS_ACCESS_KEY_ID"]
access_key_secret = os.environ["OSS_ACCESS_KEY_SECRET"]
csv_path = os.environ.get("IRIS_CSV_PATH", "/home/ray/iris.csv")
parquet_dir = os.environ.get("OSS_PARQUET_DIR", "ray-data/iris.parquet")

# 1. Read: load the Iris CSV baked into the image
df = pd.read_csv(csv_path)
ds = ray.data.from_pandas(df)
print(f"Loaded {ds.count()} rows from {csv_path}")

# 2. Write: store the dataset as parquet on OSS (S3-compatible endpoint)
fs = pfs.S3FileSystem(
    access_key=access_key_id,
    secret_key=access_key_secret,
    endpoint_override=endpoint,
    scheme="https",
    force_virtual_addressing=True,
)
# Write only once: if parquet files already exist, skip the write and read them directly
parquet_path = f"{bucket_name}/{parquet_dir}"
info = fs.get_file_info(parquet_path)
has_parquet = info.type == pfs.FileType.Directory and any(
    f.type == pfs.FileType.File and f.base_name.endswith(".parquet")
    for f in fs.get_file_info(pfs.FileSelector(parquet_path, recursive=True))
)
if has_parquet:
    print(f"Parquet output already exists at oss://{parquet_path}, skip writing")
else:
    ds.write_parquet(parquet_path, filesystem=fs)
    print(f"Wrote parquet to oss://{parquet_path}")

# 3. Read back the parquet from OSS
ds2 = ray.data.read_parquet(f"{bucket_name}/{parquet_dir}", filesystem=fs)
print(f"Read back {ds2.count()} rows from oss://{bucket_name}/{parquet_dir}")

# 4. Filter + group-by aggregation: mean of each numeric column per species
filtered = ds2.filter(lambda row: row["sepal_length"] > 5.0)
print(f"Rows with sepal_length > 5.0: {filtered.count()}")
result = filtered.groupby("species").mean()
print(result.to_pandas().to_string(index=False))
