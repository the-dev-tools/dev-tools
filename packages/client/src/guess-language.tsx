import { Match, Option, pipe, Schema } from 'effect';
import { CodeMirrorMarkupLanguage } from '~code-mirror/extensions';

export const guessLanguage = (code: string) =>
  pipe(
    Match.value(code),
    Match.when(
      (_) => pipe(_, Schema.decodeUnknownOption(Schema.parseJson()), Option.isSome),
      (): CodeMirrorMarkupLanguage => 'json',
    ),
    Match.when(
      (_) => /<\?xml|<[a-z]+:[a-z]+/i.test(_),
      (): CodeMirrorMarkupLanguage => 'xml',
    ),
    Match.when(
      (_) => /<\/?[a-z][\s\S]*>/i.test(_),
      (): CodeMirrorMarkupLanguage => 'html',
    ),
    Match.orElse((): CodeMirrorMarkupLanguage => 'text'),
  );
