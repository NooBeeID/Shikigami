import { registerPaneKind } from '../registry';
import FilePane from './FilePane.svelte';

registerPaneKind({
  kind: 'file',
  component: FilePane,
  titleFor: (tab) => {
    const base = (tab.filePath ?? '').split(/[/\\]/).filter(Boolean).pop();
    return base || tab.title || 'File';
  },
});
