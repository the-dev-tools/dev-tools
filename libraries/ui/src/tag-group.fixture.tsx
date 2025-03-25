import { Tag, TagGroup } from './tag-group';

export default (
  <TagGroup defaultSelectedKeys={['pretty']} disallowEmptySelection label='Tag Group' selectionMode='single'>
    <Tag id='pretty'>Pretty</Tag>
    <Tag id='raw'>Raw</Tag>
    <Tag id='preview'>Preview</Tag>
  </TagGroup>
);
