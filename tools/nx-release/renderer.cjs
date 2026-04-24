const DefaultChangelogRenderer = require('nx/release/changelog-renderer').default;

/**
 * Custom changelog renderer that does not surface a "⚠️ Breaking Changes" section
 * for major bumps driven by version plans. We use `major` bumps as stability-milestone
 * markers (e.g. 0.x → 1.0.0), not as explicit API-breakage signals, so listing the
 * plan body under "⚠️ Breaking Changes" is misleading to readers of the CHANGELOG
 * and GitHub release notes.
 */
class NoBreakingChangelogRenderer extends DefaultChangelogRenderer {
  async render() {
    const contents = await super.render();
    // Nx's default renderer emits "### ⚠️  Breaking Changes" for major bumps
    // driven by version plans. Rewrite it to "### 🚀 Features" so the
    // stability-milestone bump (0.x → 1.0) doesn't falsely signal API breakage.
    return contents.replace(/^### ⚠️\s+Breaking Changes$/gm, '### 🚀 Features');
  }
}

module.exports = NoBreakingChangelogRenderer;
