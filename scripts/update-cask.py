import argparse
import re
from pathlib import Path

parser = argparse.ArgumentParser(description="Update the Relaydroid Homebrew cask from published release checksums.")
parser.add_argument("tag")
parser.add_argument("checksums", type=Path)
parser.add_argument("--cask", type=Path, default=Path("Casks/relaydroid.rb"))
args = parser.parse_args()

match = re.fullmatch(r"v(\d+\.\d+\.\d+)", args.tag)
if not match:
    parser.error("the tag must be a stable vMAJOR.MINOR.PATCH version")
version = match[1]

sums = {}
for line in args.checksums.read_text(encoding="utf-8").splitlines():
    checksum = re.fullmatch(r"([0-9a-f]{64}) [ *](\S+)", line)
    if checksum:
        digest, filename = checksum.groups()
        if filename in sums:
            parser.error(f"duplicate checksum for {filename}")
        sums[filename] = digest

asset_name = "Relay" if version == "0.3.0" else "Relaydroid"
required = tuple(f"{asset_name}-macOS-{arch}.zip" for arch in ("arm64", "amd64"))
if not all(filename in sums for filename in required):
    parser.error("both macOS release archives must have SHA-256 checksums")

if args.cask.exists():
    current = re.search(r'^  version "(\d+\.\d+\.\d+)"$', args.cask.read_text(encoding="utf-8"), re.M)
    if not current:
        parser.error("could not read the existing cask version")
    if tuple(map(int, current[1].split("."))) > tuple(map(int, version.split("."))):
        print(f"Keeping cask version {current[1]}; {version} is older.")
        raise SystemExit(0)

args.cask.parent.mkdir(parents=True, exist_ok=True)
args.cask.write_text(
    f'''cask "relaydroid" do
  arch arm: "arm64", intel: "amd64"

  version "{version}"
  sha256 arm:   "{sums[required[0]]}",
         intel: "{sums[required[1]]}"

  url "https://github.com/songkwon/relaydroid/releases/download/v#{{version}}/{asset_name}-macOS-#{{arch}}.zip"
  name "Relaydroid"
  desc "Transfer files between a computer and Android devices"
  homepage "https://github.com/songkwon/relaydroid"

  depends_on macos: :sonoma

  app "{asset_name}.app"
end
''',
    encoding="utf-8",
)
