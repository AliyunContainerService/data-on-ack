#!/usr/bin/env python3
"""校验解码后的 kubeconfig 各证书字段 base64 是否完整。只输出字段级 OK/BAD，
不打印任何凭证内容。用法: kc-cert-check.py <decoded-kubeconfig-path>"""
import base64
import sys

import yaml


def chk(name, b64):
    if not b64:
        print(f"{name}: 缺失")
        return True
    try:
        base64.b64decode(b64, validate=True)
        print(f"{name}: base64 完整 OK")
        return True
    except Exception as e:  # noqa: BLE001
        print(f"{name}: base64 损坏 -> {e}")
        return False


def main() -> int:
    path = sys.argv[1] if len(sys.argv) > 1 else "/tmp/kc.yaml"
    d = yaml.safe_load(open(path, encoding="utf-8"))
    print("YAML 结构: 合法")
    ok = True
    for u in d.get("users", []) or []:
        nm = u.get("name", "?")
        ud = u.get("user", {}) or {}
        ok &= chk(f"user[{nm}].client-certificate-data", ud.get("client-certificate-data"))
        ok &= chk(f"user[{nm}].client-key-data", ud.get("client-key-data"))
    for c in d.get("clusters", []) or []:
        nm = c.get("name", "?")
        ok &= chk(f"cluster[{nm}].certificate-authority-data",
                  c.get("cluster", {}).get("certificate-authority-data"))
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
