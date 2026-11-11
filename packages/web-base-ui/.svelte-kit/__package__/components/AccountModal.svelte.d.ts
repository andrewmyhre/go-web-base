import { type Me } from '../api/index';
type $$ComponentProps = {
    user: Me;
    open: boolean;
    onclose: () => void;
};
declare const AccountModal: import("svelte").Component<$$ComponentProps, {}, "">;
type AccountModal = ReturnType<typeof AccountModal>;
export default AccountModal;
