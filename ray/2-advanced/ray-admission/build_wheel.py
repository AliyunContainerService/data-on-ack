"""Build a pure-Python wheel without external build dependencies."""

import base64
import csv
import hashlib
import io
import shutil
import zipfile
from pathlib import Path


def main():
    package = Path(__file__).resolve().parent
    dist = package / "dist"
    dist.mkdir(exist_ok=True)
    shutil.copy2(package / "README.md", dist / "README.md")
    wheel = dist / "ray_admission-0.1.0-py3-none-any.whl"
    metadata_dir = "ray_admission-0.1.0.dist-info"
    files = {
        "ray_admission/queue.py": (
            package / "src/ray_admission/queue.py"
        ).read_bytes(),
        "ray_admission/__init__.py": (
            package / "src/ray_admission/__init__.py"
        ).read_bytes(),
        "ray_admission/cli.py": (
            package / "src/ray_admission/cli.py"
        ).read_bytes(),
        f"{metadata_dir}/METADATA": (
            "Metadata-Version: 2.1\n"
            "Name: ray-admission\n"
            "Version: 0.1.0\n"
            "Summary: Experimental Ray submission admission queue and CLI\n"
            "Requires-Python: >=3.10\n"
            "Requires-Dist: ray>=2.58\n"
        ).encode(),
        f"{metadata_dir}/WHEEL": (
            "Wheel-Version: 1.0\n"
            "Generator: ray-admission/build_wheel.py\n"
            "Root-Is-Purelib: true\n"
            "Tag: py3-none-any\n"
        ).encode(),
        f"{metadata_dir}/entry_points.txt": (
            "[console_scripts]\nray-admission = ray_admission.cli:main\n"
        ).encode(),
    }
    record = io.StringIO(newline="")
    for source in (package / "src/ray_admission").rglob("*.py"):
        files[source.relative_to(package / "src").as_posix()] = source.read_bytes()
    writer = csv.writer(record)
    for name, data in files.items():
        digest = base64.urlsafe_b64encode(hashlib.sha256(data).digest()).rstrip(b"=")
        writer.writerow([name, "sha256=" + digest.decode(), len(data)])
    record_name = f"{metadata_dir}/RECORD"
    writer.writerow([record_name, "", ""])
    files[record_name] = record.getvalue().encode()
    with zipfile.ZipFile(wheel, "w", zipfile.ZIP_DEFLATED) as archive:
        for name, data in files.items():
            archive.writestr(name, data)
    print(wheel)


if __name__ == "__main__":
    main()
