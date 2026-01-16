import { AnyRouter, createLink, LinkComponentProps, RegisteredRouter } from '@tanstack/react-router';
import { HKT } from 'effect';
import { ReactNode } from 'react';

export type GenericLinkComponent<TComponentTypeLambda extends HKT.TypeLambda, TGeneric> = <
  T extends TGeneric,
  TRouter extends AnyRouter = RegisteredRouter,
  const TFrom extends string = string,
  const TTo extends string | undefined = undefined,
  const TMaskFrom extends string = TFrom,
  const TMaskTo extends string = '',
>(
  props: LinkComponentProps<
    HKT.Kind<TComponentTypeLambda, never, never, never, T>,
    TRouter,
    TFrom,
    TTo,
    TMaskFrom,
    TMaskTo
  >,
) => ReactNode;

export const createLinkGeneric = <
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  TComponent extends HKT.TypeLambda & { type: (props: any) => ReactNode },
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  TGeneric = any,
>(
  Component: TComponent['type'],
) => createLink(Component) as GenericLinkComponent<TComponent, TGeneric>;
