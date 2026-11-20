import ProfileGlyph, {type ProfileGlyphName} from './ProfileGlyph';
type Props = {title:string;subtitle:string;icon:ProfileGlyphName;theme:'warm'|'blue';onClick:()=>void};
export default function ProfileShortcutTile({title,subtitle,icon,theme,onClick}:Props) {
  return <button type="button" className={'profile-shortcut profile-shortcut--'+theme} onClick={onClick}>
    <span className="profile-shortcut__top">
      <span className={'profile-symbol profile-symbol--'+theme}><ProfileGlyph name={icon} size={24}/></span>
      <ProfileGlyph name="chevron" size={19} className="profile-chevron"/>
    </span>
    <span className="profile-shortcut__copy"><strong>{title}</strong><small>{subtitle}</small></span>
  </button>;
}
