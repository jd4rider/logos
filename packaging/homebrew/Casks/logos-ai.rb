# typed: false
# frozen_string_literal: true

cask "logos-ai" do
  version "0.0.0"
  sha256 "REPLACE_WITH_RELEASE_SHA256"

  url "https://github.com/jd4rider/logos/releases/download/v#{version}/logos-ai-macos-universal.tar.gz"
  name "Logos AI"
  desc "Bible study desktop app with bundled Logos CLI"
  homepage "https://logos-ai.online"

  app "logos-ai.app"
  binary "logos"
end
