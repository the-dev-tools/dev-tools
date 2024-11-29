/** @type {import("syncpack").RcFile} */
export default {
  dependencyTypes: ['!local'],
  sortFirst: [
    'name',
    'displayName',
    'description',
    'author',
    'version',
    'private',
    'repository',
    'type',
    'main',
    'files',
    'scripts',
    'exports',
  ],
  versionGroups: [
    {
      label: 'PNPM Workspace Version Group',
      pinVersion: 'workspace:^',
      packages: ['!the-dev-tools'],
      dependencies: ['$LOCAL'],
      dependencyTypes: ['!local'],
    },
  ],
  semverGroups: [
    {
      label: 'Production dependencies should have fixed version numbers',
      range: '',
      dependencyTypes: ['!dev', '!peer'],
      dependencies: ['**'],
      packages: ['**'],
    },
    {
      label: 'Development dependencies should have fixed minor version',
      range: '~',
      dependencyTypes: ['dev'],
      dependencies: ['**'],
      packages: ['**'],
    },
    {
      label: 'Peer dependencies should have fixed major version',
      range: '^',
      dependencyTypes: ['peer'],
      dependencies: ['**'],
      packages: ['**'],
    },
  ],
};
