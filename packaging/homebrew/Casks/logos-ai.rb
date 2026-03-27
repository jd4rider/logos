# typed: false
# frozen_string_literal: true

cask "logos-ai" do
  version "0.1.4"
  sha256 "eab8350048cf2b7e53a3c381cd93840d99efdbc3e42bbb02e32b981a997abf83"

  url "https://github.com/jd4rider/logos-releases/releases/download/v#{version}/logos-ai-macos-universal.tar.gz"
  name "Logos AI"
  desc "Bible study desktop app with bundled Logos CLI"
  homepage "https://logos-ai.online"

  app "logos-ai.app"
  binary "logos"
end
