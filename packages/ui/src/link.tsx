import { Link as AriaLink, LinkProps as AriaLinkProps } from 'react-aria-components';
import { LinkComponent, useLink, UseLinkProps } from './router';

export interface LinkProps extends AriaLinkProps {}

export const Link: LinkComponent<LinkProps> = ({ routerOptions: _, ...props }) => {
  const { onAction, ...linkProps } = useLink(props as UseLinkProps);
  return <AriaLink {...(props as LinkProps)} {...linkProps} onPress={onAction} />;
};
