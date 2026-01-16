import { createLink } from '@tanstack/react-router';
import * as RAC from 'react-aria-components';

export interface LinkProps extends RAC.LinkProps {}

export const Link = (props: LinkProps) => <RAC.Link {...props} />;

export const RouteLink = createLink(RAC.Link);
