export type Location = {
 id:string; title:string; category:string; area:string;
 latitude:number; longitude:number; imageUrl?:string;
};
export type CaseStatus = 'received'|'working'|'resolved';
export type CaseUpdate = { id:number; locationIds:string[]; comment:string; createdAt:string; acceptedAt?:string };
export type DemoCase = {
 number:number; locationIds:string[]; createdAt:string;
 status:CaseStatus; takenAt?:string; resolvedAt?:string;
 updates:CaseUpdate[];
};
export type Tab = 'catalog'|'favorites'|'requests';
