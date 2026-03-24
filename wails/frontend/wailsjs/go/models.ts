export namespace api {
	
	export class Language {
	    id: string;
	    name: string;
	    nameLocal: string;
	    script: string;
	    scriptDirection: string;
	
	    static createFrom(source: any = {}) {
	        return new Language(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.nameLocal = source["nameLocal"];
	        this.script = source["script"];
	        this.scriptDirection = source["scriptDirection"];
	    }
	}
	export class Bible {
	    id: string;
	    dblId: string;
	    abbreviation: string;
	    abbreviationLocal: string;
	    name: string;
	    nameLocal: string;
	    description: string;
	    language: Language;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new Bible(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.dblId = source["dblId"];
	        this.abbreviation = source["abbreviation"];
	        this.abbreviationLocal = source["abbreviationLocal"];
	        this.name = source["name"];
	        this.nameLocal = source["nameLocal"];
	        this.description = source["description"];
	        this.language = this.convertValues(source["language"], Language);
	        this.type = source["type"];
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
	export class Book {
	    id: string;
	    bibleId: string;
	    abbreviation: string;
	    name: string;
	    nameLong: string;
	
	    static createFrom(source: any = {}) {
	        return new Book(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.bibleId = source["bibleId"];
	        this.abbreviation = source["abbreviation"];
	        this.name = source["name"];
	        this.nameLong = source["nameLong"];
	    }
	}
	export class Chapter {
	    id: string;
	    bibleId: string;
	    bookId: string;
	    number: string;
	    position: number;
	
	    static createFrom(source: any = {}) {
	        return new Chapter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.bibleId = source["bibleId"];
	        this.bookId = source["bookId"];
	        this.number = source["number"];
	        this.position = source["position"];
	    }
	}
	export class ChapterRef {
	    id: string;
	    number: string;
	    bookId: string;
	
	    static createFrom(source: any = {}) {
	        return new ChapterRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.number = source["number"];
	        this.bookId = source["bookId"];
	    }
	}
	export class ChapterContent {
	    id: string;
	    bibleId: string;
	    bookId: string;
	    number: string;
	    reference: string;
	    content: string;
	    verseCount: number;
	    copyright: string;
	    next?: ChapterRef;
	    previous?: ChapterRef;
	
	    static createFrom(source: any = {}) {
	        return new ChapterContent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.bibleId = source["bibleId"];
	        this.bookId = source["bookId"];
	        this.number = source["number"];
	        this.reference = source["reference"];
	        this.content = source["content"];
	        this.verseCount = source["verseCount"];
	        this.copyright = source["copyright"];
	        this.next = this.convertValues(source["next"], ChapterRef);
	        this.previous = this.convertValues(source["previous"], ChapterRef);
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
	
	
	export class SearchVerse {
	    id: string;
	    orgId: string;
	    bookId: string;
	    bibleId: string;
	    chapterId: string;
	    reference: string;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new SearchVerse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.orgId = source["orgId"];
	        this.bookId = source["bookId"];
	        this.bibleId = source["bibleId"];
	        this.chapterId = source["chapterId"];
	        this.reference = source["reference"];
	        this.text = source["text"];
	    }
	}
	export class SearchData {
	    query: string;
	    limit: number;
	    offset: number;
	    total: number;
	    verseCount: number;
	    verses: SearchVerse[];
	
	    static createFrom(source: any = {}) {
	        return new SearchData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.limit = source["limit"];
	        this.offset = source["offset"];
	        this.total = source["total"];
	        this.verseCount = source["verseCount"];
	        this.verses = this.convertValues(source["verses"], SearchVerse);
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
	
	export class VerseRef {
	    id: string;
	    number: string;
	
	    static createFrom(source: any = {}) {
	        return new VerseRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.number = source["number"];
	    }
	}
	export class VerseContent {
	    id: string;
	    bookId: string;
	    chapterId: string;
	    bibleId: string;
	    reference: string;
	    content: string;
	    copyright: string;
	    next?: VerseRef;
	    previous?: VerseRef;
	
	    static createFrom(source: any = {}) {
	        return new VerseContent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.bookId = source["bookId"];
	        this.chapterId = source["chapterId"];
	        this.bibleId = source["bibleId"];
	        this.reference = source["reference"];
	        this.content = source["content"];
	        this.copyright = source["copyright"];
	        this.next = this.convertValues(source["next"], VerseRef);
	        this.previous = this.convertValues(source["previous"], VerseRef);
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

}

