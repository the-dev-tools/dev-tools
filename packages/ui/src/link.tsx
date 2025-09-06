import * as RAC from 'react-aria-components';
import { LinkComponent, useLink, UseLinkProps } from './router';

export interface LinkProps extends RAC.LinkProps {}

export const Link: LinkComponent<LinkProps> = ({ routerOptions: _, ...props }) => {
  const { onAction, ...linkProps } = useLink(props as UseLinkProps);
  return <RAC.Link {...(props as LinkProps)} {...linkProps} onPress={onAction} />;
};
