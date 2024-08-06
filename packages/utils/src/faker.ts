import { base, en, Faker as FakerClass } from '@faker-js/faker';
import { Context, Layer } from 'effect';

export class Faker extends Context.Tag('Faker')<Faker, FakerClass>() {}

export const FakerLive = Layer.sync(Faker, () => {
  const faker = new FakerClass({ locale: [en, base] });
  faker.seed(0);
  return faker;
});
