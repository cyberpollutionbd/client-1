//
//  KBService.h
//  Keybase
//
//  Created by Gabriel on 5/15/15.
//  Copyright (c) 2015 Gabriel Handford. All rights reserved.
//

#import <Foundation/Foundation.h>

#import "KBLaunchService.h"
#import "KBEnvironment.h"

@interface KBService : KBLaunchService

- (instancetype)initWithEnvironment:(KBEnvironment *)environment;

@end
