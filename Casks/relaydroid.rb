cask "relaydroid" do
  arch arm: "arm64", intel: "amd64"

  version "0.4.4"
  sha256 arm:   "88650694a97efc2cd2566d1d1dc55c1e0cb7b93be295fcceb0b1813346dfb1b4",
         intel: "72bd6b88aaec2069733a4b8883152c9e98a52ec476695fc67f26c9d294e59020"

  url "https://github.com/songkwon/relaydroid/releases/download/v#{version}/Relaydroid-macOS-#{arch}.zip"
  name "Relaydroid"
  desc "Transfer files between a computer and Android devices"
  homepage "https://github.com/songkwon/relaydroid"

  depends_on macos: :sonoma

  app "Relaydroid.app"
end
