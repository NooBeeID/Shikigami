import { registerPaneKind } from '../registry';
import FilePane from './FilePane.svelte';

registerPaneKind({
  kind: 'file',
  component: FilePane,
  titleFor: (leaf) => {
    const base = (leaf.filePath ?? '').split(/[/\\]/).filter(Boolean).pop();
    return base || leaf.title || 'File';
  },
});
