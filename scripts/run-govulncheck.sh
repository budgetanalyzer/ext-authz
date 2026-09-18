#!/usr/bin/env bash

set -uo pipefail

output_dir="${1:-govulncheck-results}"
mkdir -p "${output_dir}"

govulncheck -version >"${output_dir}/govulncheck-version.txt" 2>&1
version_status=$?

govulncheck ./... >"${output_dir}/govulncheck.txt" \
  2>"${output_dir}/govulncheck-text.stderr"
text_status=$?

govulncheck -json ./... >"${output_dir}/govulncheck.json" \
  2>"${output_dir}/govulncheck-json.stderr"
json_status=$?

json_validation_status=1
if [[ -s "${output_dir}/govulncheck.json" ]]; then
  jq -e . "${output_dir}/govulncheck.json" >/dev/null \
    2>"${output_dir}/govulncheck-json-validation.stderr"
  json_validation_status=$?
else
  echo "govulncheck JSON output is empty." \
    >"${output_dir}/govulncheck-json-validation.stderr"
fi

evidence_status=0
for evidence_file in govulncheck-version.txt govulncheck.txt; do
  if [[ ! -s "${output_dir}/${evidence_file}" ]]; then
    echo "Required evidence is empty: ${evidence_file}" >&2
    evidence_status=1
  fi
done

cat "${output_dir}/govulncheck.txt"
cat "${output_dir}/govulncheck-text.stderr" >&2
cat "${output_dir}/govulncheck-json.stderr" >&2

result="operational-error"
if [[ "${version_status}" -eq 0 && "${json_status}" -eq 0 && \
  "${json_validation_status}" -eq 0 && "${evidence_status}" -eq 0 ]]; then
  case "${text_status}" in
    0)
      result="clean"
      ;;
    3)
      result="findings"
      ;;
  esac
fi

printf '{\n  "result": "%s",\n  "version_exit_code": %d,\n  "text_exit_code": %d,\n  "json_exit_code": %d,\n  "json_validation_exit_code": %d,\n  "evidence_exit_code": %d\n}\n' \
  "${result}" "${version_status}" "${text_status}" "${json_status}" \
  "${json_validation_status}" "${evidence_status}" \
  >"${output_dir}/scan-result.json"

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    echo "## govulncheck"
    echo
    echo "Result: ${result}"
    echo
    echo "Text exit code: ${text_status}; JSON exit code: ${json_status}; JSON validation exit code: ${json_validation_status}."
  } >>"${GITHUB_STEP_SUMMARY}"
fi

if [[ "${result}" == "operational-error" ]]; then
  echo "govulncheck failed to produce complete text and JSON evidence." >&2
  exit 1
fi

if [[ "${result}" == "findings" ]]; then
  echo "govulncheck reported reachable vulnerabilities; findings are recorded but are not a merge gate." >&2
fi
