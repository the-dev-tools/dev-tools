import {
  Binder,
  For,
  getSymbolCreatorSymbol,
  Indent,
  refkey,
  Show,
  SourceDirectory,
  SourceDirectoryContext,
  useContext,
} from '@alloy-js/core';
import {
  CommaList,
  ObjectExpression,
  ObjectProperty,
  SourceFile,
  TSModuleScope,
  TSOutputSymbol,
  ValueExpression,
  VarDeclaration,
} from '@alloy-js/typescript';
import { EmitContext, Namespace } from '@typespec/compiler';
import { Output, useTsp, writeOutput } from '@typespec/emitter-framework';
import { Array, Option, pipe, Schema, String } from 'effect';
import { join } from 'node:path/posix';
import { primaryKeys, Project, projects, Projects } from '../core/index.js';
import { collections, EmitterOptions } from './lib.js';

export const $onEmit = async (context: EmitContext<(typeof EmitterOptions)['Encoded']>) => {
  const { emitterOutputDir, program } = context;

  const options = Schema.decodeSync(EmitterOptions)(context.options);

  if (program.compilerOptions.noEmit) return;

  const bindExternals = (binder: Binder) => {
    const namespaceFiles = (_: {
      namespaces: Namespace[];
      path: string;
      project: Project;
    }): { namespace: Namespace; path: string }[] =>
      pipe(
        _.namespaces.values().toArray(),
        Array.flatMap((namespace) => {
          const name = String.pascalToSnake(namespace.name);
          const path = join(_.path, name);

          const file = {
            namespace,
            path: join(path, `v${_.project.version}`, `${name}_pb.js`),
          };

          const childFiles = namespaceFiles({
            ..._,
            namespaces: namespace.namespaces.values().toArray(),
            path,
          });

          return [file, ...childFiles];
        }),
      );

    const files = pipe(
      projects(program).values().toArray(),
      Array.flatMap((_) =>
        namespaceFiles({
          namespaces: [_.namespace],
          path: join('../', options.bufTypeScriptPath),
          project: _,
        }),
      ),
    );

    Array.forEach(files, ({ namespace, path }) => {
      const scope = new TSModuleScope(path, undefined, { binder });

      new TSOutputSymbol(namespace.name + 'Service', scope.spaces, {
        binder,
        refkeys: refkey('service', namespace),
      });

      namespace.models.forEach(
        (_) =>
          new TSOutputSymbol(_.name + 'Schema', scope.spaces, {
            binder,
            refkeys: refkey('message', namespace, _.name),
          }),
      );
    });
  };

  await writeOutput(
    program,
    <Output externals={[{ [getSymbolCreatorSymbol()]: bindExternals }]} printWidth={120} program={program}>
      <Projects>
        {(_) => (
          <SourceDirectory path={`v${_.version}`}>
            <Files includeNestedSchemas namespace={_.namespace} />
          </SourceDirectory>
        )}
      </Projects>
    </Output>,
    join(emitterOutputDir, 'tanstack-db/typescript'),
  );
};

interface FilesProps {
  includeNestedSchemas?: boolean;
  namespace: Namespace;
}

const Files = ({ includeNestedSchemas, namespace }: FilesProps) => {
  const { program } = useTsp();
  const { path } = useContext(SourceDirectoryContext)!;

  const name = String.pascalToSnake(namespace.name);

  const children = pipe(namespace.namespaces.values().toArray(), (_) => (
    <Show when={_.length > 0}>
      <SourceDirectory path={name}>
        <For each={_}>{(_) => <Files namespace={_} />}</For>
      </SourceDirectory>
    </Show>
  ));

  const file = pipe(
    namespace.models.values().toArray(),
    Array.filterMap((collection) =>
      pipe(
        collections(program).get(collection),
        Option.fromNullable,
        Option.map((_) => ({ collection, options: _ })),
      ),
    ),
    (_) => (
      <SourceFile path={`${name}.ts`}>
        <For doubleHardline each={_} ender>
          {({ collection, options }) => (
            <VarDeclaration
              const
              export
              name={`${collection.name}CollectionSchema`}
              refkey={refkey('schema', collection)}
            >
              <ObjectExpression>
                <CommaList>
                  <ObjectProperty name='item' value={refkey('message', collection.namespace, collection.name)} />

                  <ObjectProperty name='keys'>
                    <ValueExpression
                      jsValue={pipe(
                        collection.properties.values().toArray(),
                        Array.filter((_) => primaryKeys(program).has(_)),
                        Array.map((_) => _.name),
                      )}
                    />{' '}
                    as const
                  </ObjectProperty>

                  <ObjectProperty name='collection'>
                    {refkey('service', collection.namespace)}
                    .method.
                    {String.uncapitalize(collection.name)}
                    Collection
                  </ObjectProperty>

                  <ObjectProperty name='sync'>
                    <ObjectExpression>
                      <CommaList>
                        <ObjectProperty name='method'>
                          {refkey('service', collection.namespace)}
                          .method.
                          {String.uncapitalize(collection.name)}
                          Sync
                        </ObjectProperty>

                        <ObjectProperty
                          name='insert'
                          value={refkey('message', collection.namespace, `${collection.name}SyncInsert`)}
                        />

                        <ObjectProperty
                          name='upsert'
                          value={refkey('message', collection.namespace, `${collection.name}SyncUpsert`)}
                        />

                        <ObjectProperty
                          name='update'
                          value={refkey('message', collection.namespace, `${collection.name}SyncUpdate`)}
                        />

                        <ObjectProperty
                          name='delete'
                          value={refkey('message', collection.namespace, `${collection.name}SyncDelete`)}
                        />
                      </CommaList>
                    </ObjectExpression>
                  </ObjectProperty>

                  <ObjectProperty name='operations'>
                    <ObjectExpression>
                      <CommaList>
                        {options.canInsert && (
                          <ObjectProperty name='insert'>
                            {refkey('service', collection.namespace)}
                            .method.
                            {String.uncapitalize(collection.name)}
                            Insert
                          </ObjectProperty>
                        )}

                        {options.canUpdate && (
                          <ObjectProperty name='update'>
                            {refkey('service', collection.namespace)}
                            .method.
                            {String.uncapitalize(collection.name)}
                            Update
                          </ObjectProperty>
                        )}

                        {options.canDelete && (
                          <ObjectProperty name='delete'>
                            {refkey('service', collection.namespace)}
                            .method.
                            {String.uncapitalize(collection.name)}
                            Delete
                          </ObjectProperty>
                        )}
                      </CommaList>
                    </ObjectExpression>
                  </ObjectProperty>
                </CommaList>
              </ObjectExpression>
            </VarDeclaration>
          )}
        </For>

        <VarDeclaration
          const
          export
          name={`schemas_${path.replaceAll('/', '_')}_${name}`}
          refkey={refkey('schemas', namespace)}
        >
          [
          <Indent hardline trailingBreak>
            <For comma each={_} enderPunctuation hardline>
              {(_) => refkey('schema', _.collection)}
            </For>

            <Show when={includeNestedSchemas}>
              {() => {
                const namespaces = pipe(namespace, function getNestedNamesapces(_): Namespace[] {
                  const namespaces = _.namespaces.values().toArray();
                  const nestedNamespaces = Array.flatMap(namespaces, getNestedNamesapces);
                  return Array.appendAll(namespaces, nestedNamespaces);
                });

                return (
                  <For comma each={namespaces} enderPunctuation hardline>
                    {(_) => <>...{refkey('schemas', _)}</>}
                  </For>
                );
              }}
            </Show>
          </Indent>
          ]
        </VarDeclaration>
      </SourceFile>
    ),
  );

  return (
    <>
      {children}
      {file}
    </>
  );
};
