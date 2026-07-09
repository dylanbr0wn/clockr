export namespace ai {
	
	export class Endpoint {
	    name: string;
	    baseUrl: string;
	    local: boolean;
	    running: boolean;
	    models?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Endpoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.baseUrl = source["baseUrl"];
	        this.local = source["local"];
	        this.running = source["running"];
	        this.models = source["models"];
	    }
	}
	export class ValidationResult {
	    ok: boolean;
	    local: boolean;
	    verdict: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ValidationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.local = source["local"];
	        this.verdict = source["verdict"];
	        this.message = source["message"];
	    }
	}

}

export namespace connection {
	
	export class Connection {
	    id: number;
	    provider: string;
	    accountLabel: string;
	    accountId: string;
	    scopes: string[];
	    status: string;
	    connectedAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Connection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.provider = source["provider"];
	        this.accountLabel = source["accountLabel"];
	        this.accountId = source["accountId"];
	        this.scopes = source["scopes"];
	        this.status = source["status"];
	        this.connectedAt = source["connectedAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}

}

export namespace main {
	
	export class AIClassification {
	    local: boolean;
	    verdict: string;
	
	    static createFrom(source: any = {}) {
	        return new AIClassification(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.local = source["local"];
	        this.verdict = source["verdict"];
	    }
	}
	export class ManualEventResult {
	    periodId: number;
	    id: number;
	
	    static createFrom(source: any = {}) {
	        return new ManualEventResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.periodId = source["periodId"];
	        this.id = source["id"];
	    }
	}

}

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
	    provider: string;
	    externalId: string;
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
	        this.provider = source["provider"];
	        this.externalId = source["externalId"];
	        this.name = source["name"];
	        this.isPrimary = source["isPrimary"];
	        this.selected = source["selected"];
	        this.defaultCategoryId = source["defaultCategoryId"];
	    }
	}
	export class CalendarSyncConfig {
	    Puller: any;
	    Connections: any;
	
	    static createFrom(source: any = {}) {
	        return new CalendarSyncConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Puller = source["Puller"];
	        this.Connections = source["Connections"];
	    }
	}
	export class Category {
	    id: number;
	    name: string;
	    description: string;
	    key: string;
	    color: string;
	    isDefaultGap: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Category(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.key = source["key"];
	        this.color = source["color"];
	        this.isDefaultGap = source["isDefaultGap"];
	    }
	}
	export class CreateCategoryInput {
	    name: string;
	    description: string;
	    key: string;
	    color: string;
	    isDefaultGap: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CreateCategoryInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.key = source["key"];
	        this.color = source["color"];
	        this.isDefaultGap = source["isDefaultGap"];
	    }
	}
	export class Interval {
	    // Go type: time
	    start: any;
	    // Go type: time
	    end: any;
	
	    static createFrom(source: any = {}) {
	        return new Interval(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.start = this.convertValues(source["start"], null);
	        this.end = this.convertValues(source["end"], null);
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
	export class DayTimeline {
	    date: string;
	    tz: string;
	    // Go type: time
	    windowStart: any;
	    // Go type: time
	    windowEnd: any;
	    events: Interval[];
	    filled: Interval[];
	    gaps: Interval[];
	    coveredHours: number;
	    gapHours: number;
	
	    static createFrom(source: any = {}) {
	        return new DayTimeline(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.tz = source["tz"];
	        this.windowStart = this.convertValues(source["windowStart"], null);
	        this.windowEnd = this.convertValues(source["windowEnd"], null);
	        this.events = this.convertValues(source["events"], Interval);
	        this.filled = this.convertValues(source["filled"], Interval);
	        this.gaps = this.convertValues(source["gaps"], Interval);
	        this.coveredHours = source["coveredHours"];
	        this.gapHours = source["gapHours"];
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
	export class Event {
	    id: number;
	    periodId: number;
	    calendarId: number;
	    provider: string;
	    externalId: string;
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
	        this.provider = source["provider"];
	        this.externalId = source["externalId"];
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
	export class EventCategoryOverlay {
	    provider: string;
	    externalId: string;
	    instanceId?: string;
	    categoryId: number;
	
	    static createFrom(source: any = {}) {
	        return new EventCategoryOverlay(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.externalId = source["externalId"];
	        this.instanceId = source["instanceId"];
	        this.categoryId = source["categoryId"];
	    }
	}
	export class EvidenceConfig {
	    Providers: any[];
	
	    static createFrom(source: any = {}) {
	        return new EvidenceConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Providers = source["Providers"];
	    }
	}
	export class ExcludeEventInput {
	    eventId: number;
	    periodId: number;
	
	    static createFrom(source: any = {}) {
	        return new ExcludeEventInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.eventId = source["eventId"];
	        this.periodId = source["periodId"];
	    }
	}
	export class ExcludeEventResult {
	    periodId: number;
	    eventId: number;
	
	    static createFrom(source: any = {}) {
	        return new ExcludeEventResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.periodId = source["periodId"];
	        this.eventId = source["eventId"];
	    }
	}
	export class GapFill {
	    id: number;
	    periodId: number;
	    day: string;
	    start: string;
	    end: string;
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
	        this.start = source["start"];
	        this.end = source["end"];
	        this.categoryId = source["categoryId"];
	        this.note = source["note"];
	        this.source = source["source"];
	    }
	}
	export class GapSuggestion {
	    category: string;
	    description: string;
	    evidenceCount: number;
	
	    static createFrom(source: any = {}) {
	        return new GapSuggestion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.category = source["category"];
	        this.description = source["description"];
	        this.evidenceCount = source["evidenceCount"];
	    }
	}
	export class IncomingEvent {
	    CalendarID: number;
	    Provider: string;
	    ExternalID: string;
	    InstanceID: string;
	    RecurringEventID: string;
	    ICalUID: string;
	    Title: string;
	    Description: string;
	    Location: string;
	    Organizer: string;
	    Attendees: Attendee[];
	    Status: string;
	    AllDay: boolean;
	    // Go type: time
	    Start?: any;
	    // Go type: time
	    End?: any;
	    StartDate: string;
	    EndDate: string;
	    OriginalTz: string;
	
	    static createFrom(source: any = {}) {
	        return new IncomingEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.CalendarID = source["CalendarID"];
	        this.Provider = source["Provider"];
	        this.ExternalID = source["ExternalID"];
	        this.InstanceID = source["InstanceID"];
	        this.RecurringEventID = source["RecurringEventID"];
	        this.ICalUID = source["ICalUID"];
	        this.Title = source["Title"];
	        this.Description = source["Description"];
	        this.Location = source["Location"];
	        this.Organizer = source["Organizer"];
	        this.Attendees = this.convertValues(source["Attendees"], Attendee);
	        this.Status = source["Status"];
	        this.AllDay = source["AllDay"];
	        this.Start = this.convertValues(source["Start"], null);
	        this.End = this.convertValues(source["End"], null);
	        this.StartDate = source["StartDate"];
	        this.EndDate = source["EndDate"];
	        this.OriginalTz = source["OriginalTz"];
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
	
	export class ManualEventDeleteInput {
	    id: number;
	    periodId: number;
	
	    static createFrom(source: any = {}) {
	        return new ManualEventDeleteInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.periodId = source["periodId"];
	    }
	}
	export class ManualEventInput {
	    periodId: number;
	    day: string;
	    startMinutes: number;
	    endMinutes: number;
	    categoryId?: number;
	    note?: string;
	
	    static createFrom(source: any = {}) {
	        return new ManualEventInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.periodId = source["periodId"];
	        this.day = source["day"];
	        this.startMinutes = source["startMinutes"];
	        this.endMinutes = source["endMinutes"];
	        this.categoryId = source["categoryId"];
	        this.note = source["note"];
	    }
	}
	export class ManualEventUpdateInput {
	    id: number;
	    periodId: number;
	    day: string;
	    startMinutes: number;
	    endMinutes: number;
	    categoryId?: number;
	    note?: string;
	
	    static createFrom(source: any = {}) {
	        return new ManualEventUpdateInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.periodId = source["periodId"];
	        this.day = source["day"];
	        this.startMinutes = source["startMinutes"];
	        this.endMinutes = source["endMinutes"];
	        this.categoryId = source["categoryId"];
	        this.note = source["note"];
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
	export class ResolveReviewItemInput {
	    reviewItemId: number;
	    action: string;
	
	    static createFrom(source: any = {}) {
	        return new ResolveReviewItemInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.reviewItemId = source["reviewItemId"];
	        this.action = source["action"];
	    }
	}
	export class ResolveReviewItemResult {
	    periodId: number;
	
	    static createFrom(source: any = {}) {
	        return new ResolveReviewItemResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.periodId = source["periodId"];
	    }
	}
	export class ReviewItem {
	    id: number;
	    periodId: number;
	    kind: string;
	    eventId?: number;
	    payload: string;
	    status: string;
	    conflictKey?: string;
	    decisionAction?: string;
	    decisionPayload?: string;
	
	    static createFrom(source: any = {}) {
	        return new ReviewItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.periodId = source["periodId"];
	        this.kind = source["kind"];
	        this.eventId = source["eventId"];
	        this.payload = source["payload"];
	        this.status = source["status"];
	        this.conflictKey = source["conflictKey"];
	        this.decisionAction = source["decisionAction"];
	        this.decisionPayload = source["decisionPayload"];
	    }
	}
	export class SyncResult {
	    added: number;
	    updated: number;
	    unchanged: number;
	    removed: number;
	    flagged: number;
	
	    static createFrom(source: any = {}) {
	        return new SyncResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.added = source["added"];
	        this.updated = source["updated"];
	        this.unchanged = source["unchanged"];
	        this.removed = source["removed"];
	        this.flagged = source["flagged"];
	    }
	}
	export class TimeWindow {
	    // Go type: time
	    start: any;
	    // Go type: time
	    end: any;
	
	    static createFrom(source: any = {}) {
	        return new TimeWindow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.start = this.convertValues(source["start"], null);
	        this.end = this.convertValues(source["end"], null);
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
	export class UpdateCategoryInput {
	    id: number;
	    name: string;
	    description: string;
	    key: string;
	    color: string;
	    isDefaultGap: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UpdateCategoryInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.key = source["key"];
	        this.color = source["color"];
	        this.isDefaultGap = source["isDefaultGap"];
	    }
	}

}

