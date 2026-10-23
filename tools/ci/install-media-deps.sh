#!/usr/bin/env bash
# GitHub's disposable Ubuntu runners only; published cloud environments use their
# saved prerequisites. Keep package signatures and the runner's source components.
set -euo pipefail

if [[ ${GITHUB_ACTIONS:-} != true || ${RUNNER_OS:-} != Linux ]]; then
  echo 'This helper is only for Linux GitHub Actions runners.' >&2
  exit 1
fi

# The runner's mirror+file source prefers Azure's HTTP mirror. Slow but continuous
# transfers there do not hit apt's inactivity timeout. Change that mirror list as
# well as direct sources, so later Playwright dependency installs benefit too.
shopt -s nullglob
for source in /etc/apt/apt-mirrors.txt /etc/apt/sources.list \
  /etc/apt/sources.list.d/*.list /etc/apt/sources.list.d/*.sources; do
  [[ -f "$source" ]] || continue
  sudo sed -E -i \
    -e 's@https?://azure\.archive\.ubuntu\.com/ubuntu([/[:space:]]|$)@https://archive.ubuntu.com/ubuntu\1@g' \
    -e 's@http://archive\.ubuntu\.com/ubuntu([/[:space:]]|$)@https://archive.ubuntu.com/ubuntu\1@g' \
    -e 's@http://security\.ubuntu\.com/ubuntu([/[:space:]]|$)@https://security.ubuntu.com/ubuntu\1@g' \
    "$source"
done

apt_options=(-o Acquire::Retries=2 -o Acquire::http::Timeout=20 -o Acquire::https::Timeout=20)
sudo apt-get "${apt_options[@]}" update
sudo env DEBIAN_FRONTEND=noninteractive apt-get "${apt_options[@]}" \
  install -y --no-install-recommends ffmpeg build-essential

# Fail here if either media tool is unusable, rather than silently skipping tests.
ffmpeg -version
ffprobe -version
