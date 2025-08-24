import type { Meta, StoryObj } from '@storybook/react-vite';

import { Tag, TagGroup } from './tag-group';

const meta = {
  component: TagGroup,
  subcomponents: { Tag },
} satisfies Meta<typeof TagGroup>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    defaultSelectedKeys: ['pretty'],
    disallowEmptySelection: true,
    label: 'Tag Group',
    selectionMode: 'single',
  },
  render: function Render(props) {
    return (
      <TagGroup {...props}>
        <Tag id='pretty'>Pretty</Tag>
        <Tag id='raw'>Raw</Tag>
        <Tag id='preview'>Preview</Tag>
      </TagGroup>
    );
  },
};
