import type { Component } from 'svelte';
import { derived } from 'svelte/store';
import { registerRailItem } from '../shell/registry';
import { registerPaneKind } from '../layout/registry';
import type { Provider } from './client';
import { reposFor } from './store';
import GitPanel from './GitPanel.svelte';
import GitIcon from './GitIcon.svelte';
import MrPane from './MrPane.svelte';
import PipelinePane from './PipelinePane.svelte';

// Rail merender komponen tanpa props; ikat provider lewat pembungkus (dibuat sekali, identitas stabil).
const bind = (C: Component<any>, provider: Provider): Component<any> =>
  ((anchor: any, props: Record<string, unknown> = {}) => (C as any)(anchor, { ...props, provider })) as unknown as Component<any>;

const gitlabPanel = bind(GitPanel, 'gitlab');
const githubPanel = bind(GitPanel, 'github');
const gitlabIcon = bind(GitIcon, 'gitlab');
const githubIcon = bind(GitIcon, 'github');

registerRailItem({
  id: 'git-gitlab',
  title: 'GitLab',
  icon: gitlabIcon,
  component: gitlabPanel,
  order: 20,
  visible: derived(reposFor('gitlab'), (rs) => rs.length > 0),
});
registerRailItem({
  id: 'git-github',
  title: 'GitHub',
  icon: githubIcon,
  component: githubPanel,
  order: 21,
  visible: derived(reposFor('github'), (rs) => rs.length > 0),
});

registerPaneKind({
  kind: 'git-mr',
  component: MrPane,
  titleFor: (leaf) => leaf.title || (leaf.number !== undefined ? '#' + leaf.number : 'Merge Request'),
});
registerPaneKind({
  kind: 'git-pipeline',
  component: PipelinePane,
  titleFor: (leaf) => leaf.title || (leaf.pipelineId ? 'Pipeline ' + leaf.pipelineId : 'Pipeline'),
});

export { copyLink, openInBrowser, sendToAgent } from './actions';
