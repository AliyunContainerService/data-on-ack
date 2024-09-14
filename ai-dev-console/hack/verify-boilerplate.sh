#!/bin/bash


files_need_boilerplate=`python ./hack/boilerplate.py`


# Run boilerplate.py unit tests
unitTestOut="$(mktemp)"
trap cleanup EXIT
cleanup() {
	rm "${unitTestOut}"
}

# Run boilerplate check
if [[ ${#files_need_boilerplate[@]} -gt 0 ]]; then
  for file in "${files_need_boilerplate[@]}"; do
    echo "Boilerplate header is wrong for: ${file}"
  done

  exit 1
fi