import { createFileRoute, useNavigate, useRouter } from '@tanstack/react-router';
import { Ulid } from 'id128';
import { ReactNode } from 'react';
import * as RAC from 'react-aria-components';
import { twJoin } from 'tailwind-merge';
import { FileKind } from '@the-dev-tools/spec/buf/api/file_system/v1/file_system_pb';
import { HttpMethod } from '@the-dev-tools/spec/buf/api/http/v1/http_pb';
import { FileCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/file_system';
import { FlowCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { HttpCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { FileImportIcon, FlowsIcon, SendRequestIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/shared/api';
import { getNextOrder } from '~/shared/lib';
import { routes } from '~/shared/routes';
import { useImportDialog } from '~/widgets/import';

export const Route = createFileRoute('/(dashboard)/(workspace)/workspace/$workspaceIdCan/')({
  component: RouteComponent,
});

function RouteComponent() {
  return (
    <div className={tw`px-4 py-16 text-center`}>
      <span className={tw`block text-xl leading-6 font-semibold tracking-tight text-on-neutral`}>
        Discover what you can do in DevTools
      </span>

      <span className={tw`block text-xs leading-5 tracking-tight text-on-neutral-low`}>
        Discover the tools to make your workflow easier and faster.
      </span>

      <div className={tw`mx-auto mt-5 flex max-w-4xl justify-center gap-4`}>
        <ImportButton />
        <NewHttpButton />
        <NewFlowButton />
      </div>
    </div>
  );
}

interface CtaButtonProps {
  description: ReactNode;
  icon: ReactNode;
  onPress: () => void;
  title: ReactNode;
}

const CtaButton = ({ description, icon, onPress, title }: CtaButtonProps) => (
  <RAC.Button
    className={tw`
      flex w-52 cursor-pointer flex-col items-center rounded-lg bg-neutral py-10 text-center transition-colors

      hover:bg-neutral-high
    `}
    onPress={onPress}
  >
    {icon}

    <span className={tw`mt-3 text-sm leading-5 font-semibold tracking-tight text-on-neutral`}>{title}</span>

    <span className={tw`text-xs leading-5 tracking-tight text-on-neutral-low`}>{description}</span>
  </RAC.Button>
);

interface CtaIconProps {
  children: ReactNode;
  className?: string;
}

const CtaIcon = ({ children, className }: CtaIconProps) => (
  <div className={twJoin(tw`rounded-full p-2 text-2xl text-on-inverse`, className)}>{children}</div>
);

const ImportButton = () => {
  const dialog = useImportDialog();

  return (
    <>
      <CtaButton
        description='Import Collections and Flows'
        icon={
          <CtaIcon className={tw`bg-amber-600`}>
            <FileImportIcon />
          </CtaIcon>
        }
        onPress={() => void dialog.open()}
        title='Import'
      />

      {dialog.render}
    </>
  );
};

const NewHttpButton = () => {
  const router = useRouter();
  const navigate = useNavigate();

  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();

  const fileCollection = useApiCollection(FileCollectionSchema);
  const httpCollection = useApiCollection(HttpCollectionSchema);

  return (
    <CtaButton
      description='Easy to test your API'
      icon={
        <CtaIcon className={tw`bg-cyan-600`}>
          <SendRequestIcon />
        </CtaIcon>
      }
      onPress={async () => {
        const httpUlid = Ulid.generate();

        httpCollection.utils.insert({
          httpId: httpUlid.bytes,
          method: HttpMethod.GET,
          name: 'New HTTP request',
        });

        fileCollection.utils.insert({
          fileId: httpUlid.bytes,
          kind: FileKind.HTTP,
          order: await getNextOrder(fileCollection),
          workspaceId,
        });

        await navigate({
          from: router.routesById[routes.dashboard.workspace.route.id].fullPath,
          params: { httpIdCan: httpUlid.toCanonical() },
          to: router.routesById[routes.dashboard.workspace.http.route.id].fullPath,
        });
      }}
      title='New HTTP Request'
    />
  );
};

const NewFlowButton = () => {
  const router = useRouter();
  const navigate = useNavigate();

  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();

  const fileCollection = useApiCollection(FileCollectionSchema);
  const flowCollection = useApiCollection(FlowCollectionSchema);

  return (
    <CtaButton
      description='Easy request with flows'
      icon={
        <CtaIcon className={tw`bg-green-600`}>
          <FlowsIcon />
        </CtaIcon>
      }
      onPress={async () => {
        const flowUlid = Ulid.generate();

        flowCollection.utils.insert({
          flowId: flowUlid.bytes,
          name: 'New flow',
          workspaceId,
        });

        fileCollection.utils.insert({
          fileId: flowUlid.bytes,
          kind: FileKind.FLOW,
          order: await getNextOrder(fileCollection),
          workspaceId,
        });

        await navigate({
          from: router.routesById[routes.dashboard.workspace.route.id].fullPath,
          params: { flowIdCan: flowUlid.toCanonical() },
          to: router.routesById[routes.dashboard.workspace.flow.route.id].fullPath,
        });
      }}
      title='New Flow'
    />
  );
};
