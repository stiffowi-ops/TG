import {ArrowLeft, BadgePercent, CheckCircle2, ChevronRight, ClipboardList, Clock3, Gift, Heart, Layers3, Link2, MapPin, Plus, Send, Settings2, UserRound} from 'lucide-react';

// Every icon is a standalone vector component. Replace one mapping without editing cards.
const icons = {
  user: UserRound,
  requests: Layers3,
  application: ClipboardList,
  settings: Settings2,
  chevron: ChevronRight,
  back: ArrowLeft,
  plus: Plus,
  gift: Gift,
  discount: BadgePercent,
  link: Link2,
  pin: MapPin,
  sent: Send,
  clock: Clock3,
  done: CheckCircle2,
  favorite: Heart,
} as const;
export type ProfileGlyphName = keyof typeof icons;

export default function ProfileGlyph({name,size=24,className=''}:{name:ProfileGlyphName;size?:number;className?:string}) {
  const Icon=icons[name];
  return <Icon size={size} strokeWidth={1.85} className={className} aria-hidden="true" focusable="false"/>;
}
