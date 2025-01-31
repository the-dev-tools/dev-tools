export const goToRoot = () => {
  if (process.cwd().endsWith('libraries/scripts')) process.chdir('../..');
  else throw new Error(`Incorrect working directory: '${process.cwd()}', expected: '.../libraries/scripts'`);
};
