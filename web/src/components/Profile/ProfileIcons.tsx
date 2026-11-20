import {
 ArrowLeft, BadgePercent, CheckCircle2, ChevronRight, ClipboardList,
 Clock3, Gift, Layers3, Link2, MapPin, Plus, Send, Settings2, UserRound
} from 'lucide-react';
import type {LucideProps} from 'lucide-react';

// A single icon catalog. Cards own no vector paths, so every symbol can be swapped here.
export const profileIcons={
 user:UserRound,
 requests:Layers3,
 case:ClipboardList,
 settings:Settings2,
 next:ChevronRight,
 back:ArrowLeft,
 location:MapPin,
 add:Plus,
 gift:Gift,
 discount:BadgePercent,
 referral:Link2,
 received:Send,
 working:Clock3,
 resolved:CheckCircle2,
} as const;

export type ProfileIconName=keyof typeof profileIcons;
export function ProfileIcon({name,...props}:LucideProps&{name:ProfileIconName}){
 const Icon=profileIcons[name];
 return <Icon {...props} aria-hidden="true"/>;
}
