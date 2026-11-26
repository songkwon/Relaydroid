cask "relaydroid" do
  arch arm: "arm64", intel: "amd64"

  version "0.3.0"
  sha256 arm:   "c4e804b9bc958bd70767a1a007ef568acd216e4cc014db738ed3c42d1ba11ce7",
         intel: "2a0e8083580f6c630e8139631c358c7cccba90075007cdb57cb51347998f8735"

  url "https://github.com/songkwon/relaydroid/releases/download/v#{version}/Relay-macOS-#{arch}.zip"
  name "Relaydroid"
  desc "Transfer files between a computer and Android devices"
  homepage "https://github.com/songkwon/relaydroid"

  depends_on macos: :sonoma

  app "Relay.app"
end
