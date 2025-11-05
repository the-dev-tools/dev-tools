import { Faker as FakerClass, base as fakerLocaleBase, en as fakerLocaleEn } from '@faker-js/faker';
import { Effect } from 'effect';

export class Faker extends Effect.Service<Faker>()('Faker', {
  sync: () => {
    const faker = new FakerClass({ locale: [fakerLocaleEn, fakerLocaleBase] });
    faker.seed(0);
    return faker;
  },
}) {}
