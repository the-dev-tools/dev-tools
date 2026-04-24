const DefaultChangelogRenderer = require('nx/release/changelog-renderer').default;

/**
 * Custom changelog renderer that does not surface a "⚠️ Breaking Changes" section
 * for major bumps driven by version plans. We use `major` bumps as stability-milestone
 * markers (e.g. 0.x → 1.0.0), not as explicit API-breakage signals, so listing the
 * plan body under "⚠️ Breaking Changes" is misleading to readers of the CHANGELOG
 * and GitHub release notes.
 */
class NoBreakingChangelogRenderer extends DefaultChangelogRenderer {
  preprocessChanges() {
    for (const change of this.relevantChanges) {
      change.isBreaking = false;
    }
    return super.preprocessChanges();
  }
}

module.exports = NoBreakingChangelogRenderer;
