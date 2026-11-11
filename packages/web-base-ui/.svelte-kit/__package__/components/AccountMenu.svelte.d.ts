import type { Me } from '../api/index';
type $$ComponentProps = {
    user: Me;
    onSignOut: () => void;
};
declare const AccountMenu: import("svelte").Component<$$ComponentProps, {}, "">;
type AccountMenu = ReturnType<typeof AccountMenu>;
export default AccountMenu;
