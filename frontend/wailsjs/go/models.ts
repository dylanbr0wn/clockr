export namespace service {
	
	export class Attendee {
	    email: string;
	    displayName?: string;
	    responseStatus?: string;
	    organizer?: boolean;
	    self?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Attendee(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.email = source["email"];
	        this.displayName = source["displayName"];
	        this.responseStatus = source["responseStatus"];
	        this.organizer = source["organizer"];
	        this.self = source["self"];
	    }
	}
	export class Calendar {
	    id: number;
	    googleCalendarId: string;
	    name: string;
	    isPrimary: boolean;
	    selected: boolean;
	    defaultCategoryId?: number;
	
	    static createFrom(source: any = {}) {
	        return new Calendar(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.googleCalendarId = source["googleCalendarId"];
	        this.name = source["name"];
	        this.isPrimary = source["isPrimary"];
	        this.selected = source["selected"];
	        this.defaultCategoryId = source["defaultCategoryId"];
	    }
	}
	export class Category {
	    id: number;
	    name: string;
	    isDefaultGap: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Category(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.isDefaultGap = source["isDefaultGap"];
	    }
	}
	export class Event {
	    id: number;
	    periodId: number;
	    calendarId: number;
	    googleEventId: string;
	    instanceId?: string;
	    recurringEventId?: string;
	    icalUid?: string;
	    title: string;
	    description?: string;
	    location?: string;
	    organizer?: string;
	    attendees: Attendee[];
	    status?: string;
	    allDay: boolean;
	    // Go type: time
	    start?: any;
	    // Go type: time
	    end?: any;
	    startDate?: string;
	    endDate?: string;
	    originalTz?: string;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Event(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.periodId = source["periodId"];
	        this.calendarId = source["calendarId"];
	        this.googleEventId = source["googleEventId"];
	        this.instanceId = source["instanceId"];
	        this.recurringEventId = source["recurringEventId"];
	        this.icalUid = source["icalUid"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.location = source["location"];
	        this.organizer = source["organizer"];
	        this.attendees = this.convertValues(source["attendees"], Attendee);
	        this.status = source["status"];
	        this.allDay = source["allDay"];
	        this.start = this.convertValues(source["start"], null);
	        this.end = this.convertValues(source["end"], null);
	        this.startDate = source["startDate"];
	        this.endDate = source["endDate"];
	        this.originalTz = source["originalTz"];
	        this.active = source["active"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class GapFill {
	    id: number;
	    periodId: number;
	    day: string;
	    // Go type: time
	    start: any;
	    // Go type: time
	    end: any;
	    categoryId?: number;
	    note?: string;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new GapFill(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.periodId = source["periodId"];
	        this.day = source["day"];
	        this.start = this.convertValues(source["start"], null);
	        this.end = this.convertValues(source["end"], null);
	        this.categoryId = source["categoryId"];
	        this.note = source["note"];
	        this.source = source["source"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Period {
	    id: number;
	    startDate: string;
	    endDate: string;
	    cadence: string;
	    anchorDate: string;
	    targetHoursPerDay: number;
	    // Go type: time
	    lastSyncedAt?: any;
	
	    static createFrom(source: any = {}) {
	        return new Period(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.startDate = source["startDate"];
	        this.endDate = source["endDate"];
	        this.cadence = source["cadence"];
	        this.anchorDate = source["anchorDate"];
	        this.targetHoursPerDay = source["targetHoursPerDay"];
	        this.lastSyncedAt = this.convertValues(source["lastSyncedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TzSegment {
	    id: number;
	    periodId: number;
	    effectiveFromDate: string;
	    ianaTz: string;
	
	    static createFrom(source: any = {}) {
	        return new TzSegment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.periodId = source["periodId"];
	        this.effectiveFromDate = source["effectiveFromDate"];
	        this.ianaTz = source["ianaTz"];
	    }
	}

}

