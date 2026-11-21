import { initGroups } from '../../lib/stores/groups';
import { registerLayoutHeader } from '../shell/registry';
import { registerSessionListWrapper, registerSessionMenuItem } from '../sidebar-left/registry';
import GroupTabs from './GroupTabs.svelte';
import GroupedSessionList from './GroupedSessionList.svelte';
import GroupSessionMenu from './GroupSessionMenu.svelte';

void initGroups();
registerLayoutHeader({ id: 'groups', component: GroupTabs, order: 0 });
registerSessionListWrapper({ id: 'groups', component: GroupedSessionList });
registerSessionMenuItem({ id: 'move-to-group', component: GroupSessionMenu, order: 50 });

export { GroupTabs, GroupedSessionList, GroupSessionMenu };
