import { Tag, TagGroup } from './tag-group';

export default (
  <TagGroup label='Tag Group' defaultSelectedKeys={['pretty']} selectionMode='single' disallowEmptySelection>
    <Tag id='pretty'>Pretty</Tag>
    <Tag id='raw'>Raw</Tag>
    <Tag id='preview'>Preview</Tag>
  </TagGroup>
);
