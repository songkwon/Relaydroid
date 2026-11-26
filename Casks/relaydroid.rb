cask "relaydroid" do
  arch arm: "arm64", intel: "amd64"

  version "0.4.0"
  sha256 arm:   "80dd2c49dfcd2ee058582fa2cc4a4fedfbac8011a84a2daf1aba2b388e4e6dcd",
         intel: "269e488b5c9498085aa2cea7edbe6483b6984faec40adb8441861de69790277f"

  url "https://github.com/songkwon/relaydroid/releases/download/v#{version}/Relaydroid-macOS-#{arch}.zip"
  name "Relaydroid"
  desc "Transfer files between a computer and Android devices"
  homepage "https://github.com/songkwon/relaydroid"

  depends_on macos: :sonoma

  app "Relaydroid.app"
end
