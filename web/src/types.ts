export type Location = {
 id:string;title:string;category:string;area:string;
 latitude?:number;longitude?:number;mapUrl?:string;imageUrl?:string;imageUrls?:string[];
 description?:string;landmarks?:string;lineStatus?:string;traffic?:string;payers?:string;opportunity?:string;conclusion?:string;
};
export type CaseStatus = 'received'|'working'|'resolved';
export type CaseUpdate = {id:number;locationIds:string[];comment:string;createdAt:string;acceptedAt?:string};
export type DemoCase = {
 number:number;locationIds:string[];createdAt:string;
 status:CaseStatus;takenAt?:string;resolvedAt?:string;updates:CaseUpdate[];
 userId?:number;username?:string;name?:string;phone?:string;company?:string;kind?:string;takenBy?:number;referrerId?:number;referrerName?:string;referralStatus?:string;
};
export type NewCaseInput = {locationId:string;kind:'new'|'existing';company:string;name:string;phone:string;referralCode?:string;skipReferral?:boolean};
export type Tab = 'home'|'catalog'|'map'|'favorites'|'requests'|'admin';

export type ClientProfile={userId:number;name:string;phone:string;company:string;kind:'new'|'existing';discountPercent:number;manualDiscountPercent?:number;referralDiscountPercent?:number;updatedAt?:string};

export type MiniReferralInfo={code:string;link:string;pendingCode?:string;pending:number;awarded:number};

export type DiscountUse = {id:number;userId:number;name:string;percent:number;status:'pending'|'used';createdAt:string;approvedAt?:string};
export type DiscountWallet = {percent:number;offer?:string;history:DiscountUse[]};
