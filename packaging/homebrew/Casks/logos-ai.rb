# typed: false
# frozen_string_literal: true

cask "logos-ai" do
  version "0.1.5"
  sha256 "98fe2b659e45e8697cd6f3aa00a1790ed60fa946d8868d08afdbf5ac4f76a1df"

  url "https://github.com/jd4rider/logos-releases/releases/download/v#{version}/logos-ai-macos-universal.tar.gz"
  name "Logos AI"
  desc "Bible study desktop app with bundled Logos CLI"
  homepage "https://logos-ai.online"

  app "logos-ai.app"
  binary "logos"
end
