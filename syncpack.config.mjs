/** @type {import("syncpack").RcFile} */
export default {
  dependencyTypes: ['!local'],
  semverGroups: [
    {
      dependencies: ['**'],
      dependencyTypes: ['!dev', '!peer'],
      label: 'Production dependencies should have fixed version numbers',
      packages: ['**'],
      range: '',
    },
    {
      dependencies: ['**'],
      dependencyTypes: ['dev'],
      label: 'Development dependencies should have fixed minor version',
      packages: ['**'],
      range: '~',
    },
    {
      dependencies: ['**'],
      dependencyTypes: ['peer'],
      label: 'Peer dependencies should have fixed major version',
      packages: ['**'],
      range: '^',
    },
  ],
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
      dependencies: ['$LOCAL'],
      dependencyTypes: ['!local'],
      label: 'PNPM Workspace Version Group',
      packages: ['!the-dev-tools'],
      pinVersion: 'workspace:^',
    },
  ],
};
