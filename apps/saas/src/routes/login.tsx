import { Schema } from '@effect/schema';
import { createFileRoute } from '@tanstack/react-router';

import { LoginPage } from '../login';

export class LoginSearch extends Schema.Class<LoginSearch>('LoginSearch')({
  redirect: Schema.optional(Schema.String),
}) {}

export const Route = createFileRoute('/login')({
  validateSearch: Schema.decodeSync(LoginSearch),
  component: LoginPage,
});
